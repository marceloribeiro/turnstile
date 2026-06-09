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
	"turnstile/internal/adapter/openai"
	"turnstile/internal/adapter/openrouter"
	"turnstile/internal/meter"
	"turnstile/internal/pricing"
	"turnstile/internal/session"
)

// stubAnthropicStream emits the Messages-API streaming sequence: message_start
// carries input_tokens, message_delta carries the final output_tokens. The proxy
// must merge the two frames into one complete usage.
func stubAnthropicStream() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		emit := func(s string) {
			fmt.Fprint(w, s)
			if fl != nil {
				fl.Flush()
			}
		}
		emit("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_abc\",\"usage\":{\"input_tokens\":25,\"output_tokens\":1}}}\n\n")
		emit("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n")
		emit("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":250}}\n\n")
		emit("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
}

// TestAnthropicStreamMergesSplitUsage routes /v1/messages to the Anthropic adapter
// and proves the proxy accumulates usage split across message_start (input) and
// message_delta (output) into one metered record.
func TestAnthropicStreamMergesSplitUsage(t *testing.T) {
	up := stubAnthropicStream()
	defer up.Close()
	store := meter.NewMemStore()
	mtr := meter.New(pricing.New(), store)
	px := New(
		adapter.NewRegistry(openrouter.New(up.URL), anthropic.New(up.URL), openai.New(up.URL)),
		session.NewResolver([]byte("test-salt")),
		mtr, nil, false, nil,
	)
	gw := httptest.NewServer(px)
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/v1/messages",
		strings.NewReader(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("X-Turnstile-Session", "amsg")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	var rec meter.Record
	for i := 0; i < 50; i++ {
		if r, ok := store.Get("sess:amsg"); ok {
			rec = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec.ID != "sess:amsg" {
		t.Fatal("anthropic stream was not metered")
	}
	if rec.PromptTokens != 25 || rec.CompletionTokens != 250 {
		t.Errorf("split usage not merged: got %d+%d, want 25+250", rec.PromptTokens, rec.CompletionTokens)
	}
}
