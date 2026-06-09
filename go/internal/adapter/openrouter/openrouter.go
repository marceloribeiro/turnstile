// Package openrouter is the lead ProviderAdapter. OpenRouter is one OpenAI-shaped
// endpoint fronting many models, with a price table and in-stream cost reporting,
// which makes it a good default and registry fallback.
package openrouter

import (
	"bytes"
	"encoding/json"
	"strings"

	"turnstile/internal/adapter"
)

// DefaultBase is OpenRouter's API origin.
const DefaultBase = "https://openrouter.ai"

// Adapter forwards to OpenRouter. Base is configurable (defaults to DefaultBase)
// so it can be pointed at a stub in tests or an alternate host.
type Adapter struct {
	base string
}

// New returns an OpenRouter adapter. An empty base uses DefaultBase.
func New(base string) *Adapter {
	if base == "" {
		base = DefaultBase
	}
	return &Adapter{base: strings.TrimRight(base, "/")}
}

func (a *Adapter) Name() string { return "openrouter" }

// Match recognises OpenRouter's OpenAI-compatible API surface. OpenRouter is also
// registered as the registry fallback, so unmatched traffic still routes here;
// Match matters once other adapters sit ahead of it.
func (a *Adapter) Match(r *adapter.Request) bool {
	return strings.HasPrefix(r.Path, "/api/v1/") || strings.HasSuffix(r.Path, "/chat/completions")
}

// ForwardURL passes the path and query through unchanged to OpenRouter.
func (a *Adapter) ForwardURL(r *adapter.Request) string {
	u := a.base + r.Path
	if r.RawQuery != "" {
		u += "?" + r.RawQuery
	}
	return u
}

// chatRequest is the subset of the OpenAI-compatible body we read for session
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

// ExtractRequestMeta reads model, native user, and the conversation anchor
// (first system + first user message) from an OpenAI-shaped body.
func (a *Adapter) ExtractRequestMeta(r *adapter.Request) adapter.RequestMeta {
	var body chatRequest
	if err := json.Unmarshal(r.Body, &body); err != nil {
		return adapter.RequestMeta{}
	}

	user := body.User
	if user == "" {
		user = body.Metadata.UserID // Anthropic-style fallback; harmless here
	}

	var sys, firstUser string
	for _, m := range body.Messages {
		if m.Role == "system" && sys == "" {
			sys = contentText(m.Content)
		}
		if m.Role == "user" && firstUser == "" {
			firstUser = contentText(m.Content)
		}
	}
	anchor := ""
	if sys != "" || firstUser != "" {
		anchor = sys + "\x00" + firstUser
	}

	return adapter.RequestMeta{Model: body.Model, User: user, Anchor: anchor}
}

// ParseUsage extracts token counts and OpenRouter's reported cost from a streamed
// SSE data frame. OpenRouter emits usage (with `cost`) in the terminal frame when
// usage accounting is requested (validated in Phase 0).
func (a *Adapter) ParseUsage(sseData []byte) (adapter.Usage, bool) {
	if bytes.Equal(bytes.TrimSpace(sseData), []byte("[DONE]")) {
		return adapter.Usage{}, false
	}
	var chunk struct {
		Usage *struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			Cost             float64 `json:"cost"`
		} `json:"usage"`
	}
	if json.Unmarshal(sseData, &chunk) != nil || chunk.Usage == nil {
		return adapter.Usage{}, false
	}
	return adapter.Usage{
		PromptTokens:     chunk.Usage.PromptTokens,
		CompletionTokens: chunk.Usage.CompletionTokens,
		ReportedCost:     chunk.Usage.Cost,
		HasReportedCost:  chunk.Usage.Cost > 0,
	}, true
}

// NativeError renders an OpenAI-shaped error body with HTTP 429, so an app's
// existing rate-limit/error handling catches a Turnstile block transparently.
func (a *Adapter) NativeError(reason string) (int, string, []byte) {
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": "Turnstile blocked this request: " + reason,
			"type":    "turnstile_blocked",
			"code":    reason,
		},
	})
	return 429, "application/json", body
}

// contentText returns string content directly, or the raw JSON for structured
// (e.g. multimodal) content — either way a value stable across conversation turns.
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}
