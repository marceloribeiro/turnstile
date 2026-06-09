// Package adapter defines the ProviderAdapter abstraction: one implementation
// per provider wire format (OpenRouter, OpenAI, Anthropic, …), behind a common
// interface the data-plane proxy routes through. New providers are added by
// implementing Adapter and registering it ahead of the fallback.
package adapter

import "net/http"

// Request is the lightweight, read-only view the proxy hands to adapters for
// routing decisions. We deliberately don't pass *http.Request so adapters can't
// touch connection state, and so it's explicit what they may inspect.
type Request struct {
	Method   string
	Path     string
	RawQuery string
	Host     string
	Header   http.Header
	Body     []byte
}

// RequestMeta is the provider-agnostic information the session resolver needs,
// extracted from a provider-specific request body by the adapter (M3).
type RequestMeta struct {
	// Model is the requested model id.
	Model string
	// Anchor is the stable conversation root (system prompt + first user message)
	// used for anchor-hash session inference. Empty when not extractable.
	Anchor string
	// User is the native end-user id (OpenAI `user` / Anthropic metadata.user_id),
	// linked for rollups. Empty if absent.
	User string
	// PreviousResponseID links a turn to its predecessor on the OpenAI Responses
	// API (M8). Empty for stateless APIs.
	PreviousResponseID string
}

// Usage is the token accounting (and, where the provider reports it, the cost)
// for one completed call. ReportedCost is authoritative when HasReportedCost is
// set — OpenRouter returns it in the stream (validated in Phase 0).
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	ReportedCost     float64
	HasReportedCost  bool
	// ResponseID is the provider's id for this response, when the adapter can read
	// it. Used to link an OpenAI Responses-API turn to the next one that references
	// it via previous_response_id. Empty for adapters/providers without it.
	ResponseID string
}

// Adapter encapsulates one provider's wire specifics.
type Adapter interface {
	// Name identifies the adapter (logs, metrics).
	Name() string
	// Match reports whether this adapter handles the request.
	Match(r *Request) bool
	// ForwardURL returns the complete upstream URL (incl. path + query) to
	// forward to. Returning the full URL — rather than just a base — lets future
	// adapters rewrite paths (e.g. Gemini's model-in-URL, Anthropic's /v1/messages).
	ForwardURL(r *Request) string
	// ExtractRequestMeta pulls session-relevant fields out of the request body.
	ExtractRequestMeta(r *Request) RequestMeta
	// ParseUsage extracts usage from a single SSE data payload (the bytes after
	// "data: "). Returns false when the frame carries no usage. The proxy calls it
	// only on candidate frames, off the byte-forwarding critical path.
	ParseUsage(sseData []byte) (Usage, bool)
	// NativeError renders an enforcement block as an error shaped like THIS
	// provider's errors, so the customer's existing error handling catches it.
	NativeError(reason string) (status int, contentType string, body []byte)
}

// Registry routes a request to the first matching adapter, falling back to a
// default when none match. The fallback is the OpenRouter adapter, so unmatched
// traffic still routes somewhere; provider-specific adapters Match by host/path
// ahead of the fallback.
type Registry struct {
	adapters []Adapter
	fallback Adapter
}

// NewRegistry builds a registry with a fallback (may be nil) and zero or more
// match-first adapters.
func NewRegistry(fallback Adapter, adapters ...Adapter) *Registry {
	return &Registry{adapters: adapters, fallback: fallback}
}

// Pick returns the adapter for a request, or nil if none match and there is no
// fallback.
func (reg *Registry) Pick(r *Request) Adapter {
	for _, a := range reg.adapters {
		if a.Match(r) {
			return a
		}
	}
	return reg.fallback
}
