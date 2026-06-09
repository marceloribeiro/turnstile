package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"turnstile/internal/adapter"
	"turnstile/internal/adapter/openai"
	"turnstile/internal/adapter/openrouter"
	"turnstile/internal/meter"
	"turnstile/internal/pricing"
	"turnstile/internal/session"
)

// stubResponses streams an OpenAI Responses-API `response.completed` event with an
// incrementing response id, so a test can chain turns via previous_response_id.
func stubResponses() *httptest.Server {
	var n int64
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := atomic.AddInt64(&n, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_%d\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}}\n\n", id)
		if fl != nil {
			fl.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		if fl != nil {
			fl.Flush()
		}
	}))
}

// TestOpenAIResponsesChainSharesSession drives two Responses-API turns through the
// real proxy: the second references the first via previous_response_id. The proxy
// must record the response id → session link on turn 1 and resolve turn 2 to the
// same trajectory, so both turns meter into a single session.
func TestOpenAIResponsesChainSharesSession(t *testing.T) {
	up := stubResponses()
	defer up.Close()
	store := meter.NewMemStore()
	mtr := meter.New(pricing.New(), store)
	px := New(
		adapter.NewRegistry(openrouter.New(up.URL), openai.New(up.URL)),
		session.NewResolver([]byte("test-salt")),
		mtr, nil, false, nil,
	)
	gw := httptest.NewServer(px)
	defer gw.Close()

	post := func(body string) {
		req, _ := http.NewRequest(http.MethodPost, gw.URL+"/v1/responses", strings.NewReader(body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body) // read to EOF → handler returned → link recorded
		resp.Body.Close()
	}

	// Turn 1: no previous id → inferred from instructions+input. Stub returns resp_1.
	post(`{"model":"gpt-4o","instructions":"be brief","input":"start"}`)
	// Turn 2: references resp_1 with a different input → must join turn 1's session.
	post(`{"model":"gpt-4o","input":"continue","previous_response_id":"resp_1"}`)

	// Metering is async — poll until both turns are recorded against one session.
	var sessions []meter.Record
	for i := 0; i < 100; i++ {
		sessions = store.Sessions()
		if len(sessions) == 1 && sessions[0].Requests == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(sessions) != 1 {
		t.Fatalf("chain should produce exactly one session, got %d: %+v", len(sessions), sessions)
	}
	s := sessions[0]
	if !strings.HasPrefix(s.ID, "infer:") {
		t.Errorf("expected an inferred session id, got %q", s.ID)
	}
	if s.Requests != 2 || s.PromptTokens != 20 || s.CompletionTokens != 10 {
		t.Errorf("chain metering wrong (want 2 req, 20+10 tok): %+v", s.Spend)
	}
}
