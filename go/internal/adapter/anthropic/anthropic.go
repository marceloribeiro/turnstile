// Package anthropic is the direct Anthropic ProviderAdapter for the Messages API
// (/v1/messages). It matches by the /messages path and Anthropic's distinctive
// headers (anthropic-version / x-api-key), so it must be registered ahead of the
// OpenAI adapter (which broadly claims /v1/).
//
// Anthropic splits streaming usage across events — input_tokens in message_start,
// output_tokens in message_delta — which the proxy accumulates by merging the
// non-zero fields of each parsed frame.
package anthropic

import (
	"encoding/json"
	"strings"

	"turnstile/internal/adapter"
)

// DefaultBase is Anthropic's API origin.
const DefaultBase = "https://api.anthropic.com"

// Adapter forwards to Anthropic. Base is configurable (defaults to DefaultBase)
// so it can be pointed at a stub in tests.
type Adapter struct {
	base string
}

// New returns an Anthropic adapter. An empty base uses DefaultBase.
func New(base string) *Adapter {
	if base == "" {
		base = DefaultBase
	}
	return &Adapter{base: strings.TrimRight(base, "/")}
}

func (a *Adapter) Name() string { return "anthropic" }

// Match claims the Messages API. It keys off Anthropic's required version header
// (or the x-api-key auth header) and the /messages path — none of which appear on
// OpenAI traffic. Register ahead of the OpenAI adapter, which broadly claims /v1/.
func (a *Adapter) Match(r *adapter.Request) bool {
	if r.Header != nil {
		if r.Header.Get("anthropic-version") != "" || r.Header.Get("x-api-key") != "" {
			return true
		}
	}
	return strings.HasSuffix(r.Path, "/messages")
}

// ForwardURL passes the path and query through unchanged to Anthropic.
func (a *Adapter) ForwardURL(r *adapter.Request) string {
	u := a.base + r.Path
	if r.RawQuery != "" {
		u += "?" + r.RawQuery
	}
	return u
}

// messagesRequest is the subset of the Messages API body we read for session
// resolution. We never mutate or re-emit it — pure extraction.
type messagesRequest struct {
	Model    string          `json:"model"`
	System   json.RawMessage `json:"system"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	Metadata struct {
		UserID string `json:"user_id"`
	} `json:"metadata"`
}

// ExtractRequestMeta reads model, the native user (metadata.user_id), and the
// conversation anchor (top-level system + first user message). Anthropic has no
// stateful response-chain id, so PreviousResponseID is always empty.
func (a *Adapter) ExtractRequestMeta(r *adapter.Request) adapter.RequestMeta {
	var req messagesRequest
	if err := json.Unmarshal(r.Body, &req); err != nil {
		return adapter.RequestMeta{}
	}
	var firstUser string
	for _, m := range req.Messages {
		if m.Role == "user" {
			firstUser = contentText(m.Content)
			break
		}
	}
	return adapter.RequestMeta{
		Model:  req.Model,
		User:   req.Metadata.UserID,
		Anchor: anchorOf(contentText(req.System), firstUser),
	}
}

func anchorOf(system, firstUser string) string {
	if system == "" && firstUser == "" {
		return ""
	}
	return system + "\x00" + firstUser
}

type usageFields struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ParseUsage extracts token counts and the message id from an Anthropic payload.
// It handles the non-streaming message object (top-level usage), the streaming
// message_start event (usage nested under `message`, carrying input_tokens), and
// the message_delta event (top-level usage, carrying the final output_tokens).
// Anthropic reports no cost, so pricing comes from the table.
func (a *Adapter) ParseUsage(sseData []byte) (adapter.Usage, bool) {
	var frame struct {
		ID      string       `json:"id"`
		Usage   *usageFields `json:"usage"`
		Message *struct {
			ID    string       `json:"id"`
			Usage *usageFields `json:"usage"`
		} `json:"message"`
	}
	if json.Unmarshal(sseData, &frame) != nil {
		return adapter.Usage{}, false
	}
	u, respID := frame.Usage, frame.ID
	if u == nil && frame.Message != nil {
		u, respID = frame.Message.Usage, frame.Message.ID
	}
	if u == nil {
		return adapter.Usage{}, false
	}
	return adapter.Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		ResponseID:       respID,
	}, true
}

// NativeError renders an Anthropic-shaped error body with HTTP 429, so an app's
// existing error handling catches a Turnstile block transparently.
func (a *Adapter) NativeError(reason string) (int, string, []byte) {
	body, _ := json.Marshal(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "turnstile_blocked",
			"message": "Turnstile blocked this request: " + reason,
		},
	})
	return 429, "application/json", body
}

// contentText returns string content directly, concatenates the text parts of a
// structured content array (Anthropic content blocks / system blocks), or falls
// back to the raw JSON — a value stable across conversation turns.
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
