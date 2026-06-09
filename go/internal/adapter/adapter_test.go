package adapter

import (
	"net/http"
	"strings"
	"testing"
)

// stubAdapter matches requests whose path has a given prefix.
type stubAdapter struct {
	name   string
	prefix string
}

func (s stubAdapter) Name() string                              { return s.name }
func (s stubAdapter) Match(r *Request) bool                     { return strings.HasPrefix(r.Path, s.prefix) }
func (s stubAdapter) ForwardURL(r *Request) string              { return "http://" + s.name + r.Path }
func (s stubAdapter) ExtractRequestMeta(r *Request) RequestMeta { return RequestMeta{} }
func (s stubAdapter) ParseUsage(sseData []byte) (Usage, bool)   { return Usage{}, false }
func (s stubAdapter) NativeError(reason string) (int, string, []byte) {
	return 429, "application/json", []byte(`{"error":"` + reason + `"}`)
}

func req(path string) *Request {
	return &Request{Method: http.MethodPost, Path: path, Header: http.Header{}}
}

func TestRegistryPicksMatchBeforeFallback(t *testing.T) {
	fallback := stubAdapter{name: "fallback", prefix: ""}
	openai := stubAdapter{name: "openai", prefix: "/openai/"}
	reg := NewRegistry(fallback, openai)

	if got := reg.Pick(req("/openai/v1/chat")).Name(); got != "openai" {
		t.Errorf("matched adapter: got %q, want openai", got)
	}
	if got := reg.Pick(req("/api/v1/chat/completions")).Name(); got != "fallback" {
		t.Errorf("unmatched should hit fallback: got %q, want fallback", got)
	}
}

func TestRegistryNilWhenNoMatchNoFallback(t *testing.T) {
	reg := NewRegistry(nil, stubAdapter{name: "openai", prefix: "/openai/"})
	if a := reg.Pick(req("/something/else")); a != nil {
		t.Errorf("expected nil, got %q", a.Name())
	}
}
