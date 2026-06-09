// Package gemini is the direct Google Gemini ProviderAdapter for the Generative
// Language API (/v1beta/models/{model}:generateContent and :streamGenerateContent).
//
// Gemini is the odd one out: the model lives in the URL (not the body), auth is a
// ?key= query param or an x-goog-api-key header (not Authorization), usage is
// reported as `usageMetadata` (promptTokenCount / candidatesTokenCount), and the
// content shape is `contents[].parts[].text`. It matches by the :generateContent
// action and the x-goog-api-key header, and must be registered ahead of the
// OpenAI adapter (which broadly claims /v1/, including /v1/models/...).
package gemini

import (
	"encoding/json"
	"strings"

	"turnstile/internal/adapter"
)

// DefaultBase is the Generative Language API origin.
const DefaultBase = "https://generativelanguage.googleapis.com"

// Adapter forwards to Gemini. Base is configurable (defaults to DefaultBase) so it
// can be pointed at a stub in tests.
type Adapter struct {
	base string
}

// New returns a Gemini adapter. An empty base uses DefaultBase.
func New(base string) *Adapter {
	if base == "" {
		base = DefaultBase
	}
	return &Adapter{base: strings.TrimRight(base, "/")}
}

func (a *Adapter) Name() string { return "gemini" }

// Match claims the generateContent actions and the x-goog-api-key header — none of
// which appear on OpenAI/Anthropic traffic. Register ahead of the OpenAI adapter,
// which broadly claims /v1/ (and Gemini's stable surface is /v1/models/...).
func (a *Adapter) Match(r *adapter.Request) bool {
	if r.Header != nil && r.Header.Get("x-goog-api-key") != "" {
		return true
	}
	return strings.Contains(r.Path, ":generateContent") || strings.Contains(r.Path, ":streamGenerateContent")
}

// ForwardURL passes the path and query through unchanged. Gemini's ?key= auth and
// the model-in-path both ride along untouched (the key is forwarded, never logged).
func (a *Adapter) ForwardURL(r *adapter.Request) string {
	u := a.base + r.Path
	if r.RawQuery != "" {
		u += "?" + r.RawQuery
	}
	return u
}

type geminiParts struct {
	Parts []struct {
		Text string `json:"text"`
	} `json:"parts"`
}

func (p geminiParts) text() string {
	var b strings.Builder
	for _, part := range p.Parts {
		b.WriteString(part.Text)
	}
	return b.String()
}

// generateRequest is the subset of the Gemini body we read. The model is NOT here
// — it's in the URL path — so ExtractRequestMeta also parses the path.
type generateRequest struct {
	SystemInstruction *geminiParts `json:"systemInstruction"`
	Contents          []struct {
		Role string `json:"role"`
		geminiParts
	} `json:"contents"`
}

// ExtractRequestMeta reads the model from the URL path and the conversation anchor
// (systemInstruction + first user turn) from the body. Gemini has no native user
// id or response-chain id, so both are empty.
func (a *Adapter) ExtractRequestMeta(r *adapter.Request) adapter.RequestMeta {
	meta := adapter.RequestMeta{Model: modelFromPath(r.Path)}
	var req generateRequest
	if err := json.Unmarshal(r.Body, &req); err != nil {
		return meta
	}
	var sys string
	if req.SystemInstruction != nil {
		sys = req.SystemInstruction.text()
	}
	var firstUser string
	for _, c := range req.Contents {
		if c.Role == "user" || c.Role == "" {
			firstUser = c.text()
			break
		}
	}
	if sys != "" || firstUser != "" {
		meta.Anchor = sys + "\x00" + firstUser
	}
	return meta
}

// modelFromPath pulls "{model}" out of /v1beta/models/{model}:generateContent.
func modelFromPath(path string) string {
	i := strings.Index(path, "/models/")
	if i < 0 {
		return ""
	}
	rest := path[i+len("/models/"):]
	if j := strings.IndexAny(rest, ":/"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// ParseUsage reads Gemini's usageMetadata (promptTokenCount / candidatesTokenCount)
// from a non-streaming response or an SSE chunk. Gemini reports no cost, so pricing
// comes from the table.
func (a *Adapter) ParseUsage(sseData []byte) (adapter.Usage, bool) {
	var frame struct {
		ResponseID    string `json:"responseId"`
		UsageMetadata *struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if json.Unmarshal(sseData, &frame) != nil || frame.UsageMetadata == nil {
		return adapter.Usage{}, false
	}
	return adapter.Usage{
		PromptTokens:     frame.UsageMetadata.PromptTokenCount,
		CompletionTokens: frame.UsageMetadata.CandidatesTokenCount,
		ResponseID:       frame.ResponseID,
	}, true
}

// NativeError renders a Google-API-shaped error body with HTTP 429, so an app's
// existing error handling catches a Turnstile block transparently.
func (a *Adapter) NativeError(reason string) (int, string, []byte) {
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":    429,
			"message": "Turnstile blocked this request: " + reason,
			"status":  "turnstile_blocked",
		},
	})
	return 429, "application/json", body
}
