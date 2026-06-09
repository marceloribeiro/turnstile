package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"turnstile/internal/adapter"
	"turnstile/internal/adapter/anthropic"
	"turnstile/internal/adapter/gemini"
	"turnstile/internal/adapter/openai"
	"turnstile/internal/adapter/openrouter"
	"turnstile/internal/meter"
	"turnstile/internal/pricing"
	"turnstile/internal/session"
)

// stubGeminiStream emits a content chunk then a final chunk carrying
// usageMetadata — the Gemini streamGenerateContent?alt=sse shape.
func stubGeminiStream() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		emit := func(s string) {
			fmt.Fprint(w, s)
			if fl != nil {
				fl.Flush()
			}
		}
		emit("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hi\"}]}}]}\n\n")
		emit("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" there\"}]}}],\"usageMetadata\":{\"promptTokenCount\":12,\"candidatesTokenCount\":34,\"totalTokenCount\":46}}\n\n")
	}))
}

// TestGeminiStreamMeters routes a streamGenerateContent call to the Gemini adapter
// and proves the proxy meters Gemini's usageMetadata (which the broadened usage
// marker now lets through) and resolves the model from the URL path.
func TestGeminiStreamMeters(t *testing.T) {
	up := stubGeminiStream()
	defer up.Close()
	store := meter.NewMemStore()
	mtr := meter.New(pricing.New(), store)
	px := New(
		adapter.NewRegistry(
			openrouter.New(up.URL),
			anthropic.New(up.URL),
			gemini.New(up.URL),
			openai.New(up.URL),
		),
		session.NewResolver([]byte("test-salt")),
		mtr, nil, false, nil,
	)
	gw := httptest.NewServer(px)
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost,
		gw.URL+"/v1beta/models/gemini-1.5-flash:streamGenerateContent?alt=sse",
		strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	req.Header.Set("X-Turnstile-Session", "gmsg")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	var rec meter.Record
	for i := 0; i < 50; i++ {
		if r, ok := store.Get("sess:gmsg"); ok {
			rec = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec.ID != "sess:gmsg" {
		t.Fatal("gemini stream was not metered")
	}
	if rec.PromptTokens != 12 || rec.CompletionTokens != 34 {
		t.Errorf("usageMetadata not metered: got %d+%d, want 12+34", rec.PromptTokens, rec.CompletionTokens)
	}
	if rec.Model != "gemini-1.5-flash" {
		t.Errorf("model should come from the URL path, got %q", rec.Model)
	}
}
