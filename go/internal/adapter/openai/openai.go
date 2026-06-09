// Package openai is the direct OpenAI ProviderAdapter. It handles both the
// Chat Completions API (/v1/chat/completions) and the Responses API
// (/v1/responses), including linking a turn to its predecessor via
// previous_response_id (the proxy records the response id → session on
// completion, and the session resolver follows it on the next turn).
//
// It matches the /v1/ surface, distinct from OpenRouter's /api/v1/, so a client
// pointed at OpenAI routes here while OpenRouter traffic falls through to the
// fallback adapter.
package openai

import (
	"bytes"
	"encoding/json"
	"strings"

	"turnstile/internal/adapter"
)

// DefaultBase is OpenAI's API origin.
const DefaultBase = "https://api.openai.com"

// Adapter forwards to OpenAI. Base is configurable (defaults to DefaultBase) so
// it can be pointed at a stub in tests or a compatible gateway.
type Adapter struct {
	base string
}

// New returns an OpenAI adapter. An empty base uses DefaultBase.
func New(base string) *Adapter {
	if base == "" {
		base = DefaultBase
	}
	return &Adapter{base: strings.TrimRight(base, "/")}
}

func (a *Adapter) Name() string { return "openai" }

// Match claims OpenAI's API surface (/v1/...). OpenRouter uses /api/v1/, so the
// two never collide; this adapter is registered ahead of the OpenRouter fallback.
func (a *Adapter) Match(r *adapter.Request) bool {
	return strings.HasPrefix(r.Path, "/v1/")
}

// ForwardURL passes the path and query through unchanged to OpenAI.
func (a *Adapter) ForwardURL(r *adapter.Request) string {
	u := a.base + r.Path
	if r.RawQuery != "" {
		u += "?" + r.RawQuery
	}
	return u
}

func isResponses(path string) bool { return strings.HasSuffix(path, "/responses") }

// chatRequest is the subset of the Chat Completions body we read for session
// resolution. We never mutate or re-emit it — pure extraction.
type chatRequest struct {
	Model    string `json:"model"`
	User     string `json:"user"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	Metadata struct {
		UserID string `json:"user_id"`
	} `json:"metadata"`
}

// responsesRequest is the subset of the Responses API body we read.
type responsesRequest struct {
	Model              string          `json:"model"`
	User               string          `json:"user"`
	PreviousResponseID string          `json:"previous_response_id"`
	Instructions       string          `json:"instructions"`
	Input              json.RawMessage `json:"input"`
	Metadata           struct {
		UserID string `json:"user_id"`
	} `json:"metadata"`
}

// ExtractRequestMeta reads model, native user, the conversation anchor, and (for
// the Responses API) previous_response_id from an OpenAI request body.
func (a *Adapter) ExtractRequestMeta(r *adapter.Request) adapter.RequestMeta {
	if isResponses(r.Path) {
		return extractResponses(r.Body)
	}
	return extractChat(r.Body)
}

func extractChat(body []byte) adapter.RequestMeta {
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return adapter.RequestMeta{}
	}
	user := req.User
	if user == "" {
		user = req.Metadata.UserID
	}
	var sys, firstUser string
	for _, m := range req.Messages {
		if m.Role == "system" && sys == "" {
			sys = contentText(m.Content)
		}
		if m.Role == "user" && firstUser == "" {
			firstUser = contentText(m.Content)
		}
	}
	return adapter.RequestMeta{Model: req.Model, User: user, Anchor: anchorOf(sys, firstUser)}
}

func extractResponses(body []byte) adapter.RequestMeta {
	var req responsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return adapter.RequestMeta{}
	}
	user := req.User
	if user == "" {
		user = req.Metadata.UserID
	}
	return adapter.RequestMeta{
		Model:              req.Model,
		User:               user,
		Anchor:             anchorOf(req.Instructions, firstInputText(req.Input)),
		PreviousResponseID: req.PreviousResponseID,
	}
}

func anchorOf(system, firstUser string) string {
	if system == "" && firstUser == "" {
		return ""
	}
	return system + "\x00" + firstUser
}

// usageFields covers both Chat Completions (prompt/completion) and the Responses
// API (input/output) token names.
type usageFields struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
}

// ParseUsage extracts token counts and the response id from an OpenAI payload.
// It handles three shapes: a Chat Completions body/usage chunk (top-level usage,
// prompt/completion tokens), a non-streaming Responses object (top-level usage,
// input/output tokens), and a streaming `response.completed` event (usage nested
// under `response`). OpenAI does not report cost, so pricing comes from the table.
func (a *Adapter) ParseUsage(sseData []byte) (adapter.Usage, bool) {
	if bytes.Equal(bytes.TrimSpace(sseData), []byte("[DONE]")) {
		return adapter.Usage{}, false
	}
	var frame struct {
		ID       string       `json:"id"`
		Usage    *usageFields `json:"usage"`
		Response *struct {
			ID    string       `json:"id"`
			Usage *usageFields `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal(sseData, &frame) != nil {
		return adapter.Usage{}, false
	}

	u, respID := frame.Usage, frame.ID
	if u == nil && frame.Response != nil {
		u, respID = frame.Response.Usage, frame.Response.ID
	}
	if u == nil {
		return adapter.Usage{}, false
	}

	prompt := u.PromptTokens
	if prompt == 0 {
		prompt = u.InputTokens
	}
	completion := u.CompletionTokens
	if completion == 0 {
		completion = u.OutputTokens
	}
	return adapter.Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		ResponseID:       respID,
	}, true
}

// NativeError renders an OpenAI-shaped error body with HTTP 429, so an app's
// existing error handling catches a Turnstile block transparently.
func (a *Adapter) NativeError(reason string) (int, string, []byte) {
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": "Turnstile blocked this request: " + reason,
			"type":    "turnstile_blocked",
			"code":    reason,
			"param":   nil,
		},
	})
	return 429, "application/json", body
}

// contentText returns string content directly, concatenates the text parts of a
// structured (multimodal) content array, or falls back to the raw JSON — either
// way a value stable across conversation turns.
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		if b.Len() > 0 {
			return b.String()
		}
	}
	return string(raw)
}

// firstInputText pulls the text of the first Responses-API input item. Input may
// be a plain string or an array of items ({role, content}); either form yields a
// stable anchor for the first turn of a conversation.
func firstInputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		var item struct {
			Text    string          `json:"text"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(arr[0], &item) == nil {
			if item.Text != "" {
				return item.Text
			}
			if len(item.Content) > 0 {
				return contentText(item.Content)
			}
		}
		return string(arr[0])
	}
	return string(raw)
}
