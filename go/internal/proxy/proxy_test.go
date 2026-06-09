package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"turnstile/internal/adapter"
	"turnstile/internal/adapter/openrouter"
	"turnstile/internal/enforce"
	"turnstile/internal/meter"
	"turnstile/internal/metrics"
	"turnstile/internal/pricing"
	"turnstile/internal/session"
)

// newProxy wires an OpenRouter adapter (pointed at the stub) into a registry,
// with a session resolver — the construction path used across these tests.
// metering and enforcer are optional (nil for plumbing-only tests).
func newProxy(upstreamURL string, sink metrics.Sink, metering Metering, enforcer Enforcer) *Proxy {
	return New(
		adapter.NewRegistry(openrouter.New(upstreamURL)),
		session.NewResolver([]byte("test-salt")),
		metering, enforcer,
		false, sink,
	)
}

// stubUpstream streams a few SSE chunks instantly — no artificial delay — so
// that any latency the proxy adds IS our overhead, with no provider jitter.
func stubUpstream() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		for i := 0; i < 5; i++ {
			fmt.Fprintf(w, "data: {\"chunk\":%d}\n\n", i)
			if fl != nil {
				fl.Flush()
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		if fl != nil {
			fl.Flush()
		}
	}))
}

// TestPassthrough verifies the proxy faithfully streams the upstream body back.
func TestPassthrough(t *testing.T) {
	up := stubUpstream()
	defer up.Close()
	gw := httptest.NewServer(newProxy(up.URL, nil, nil, nil))
	defer gw.Close()

	resp, err := http.Post(gw.URL+"/api/v1/chat/completions", "application/json", strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "[DONE]") || !strings.Contains(string(b), `"chunk":0`) {
		t.Fatalf("expected streamed SSE body, got %q", b)
	}
}

// TestLatencyGate is the Q12 gate: against an instant stub, end-to-end TTFT
// through the proxy must stay within p50 < 5ms, p99 < 15ms.
func TestLatencyGate(t *testing.T) {
	up := stubUpstream()
	defer up.Close()
	rec := metrics.NewRecorder(10000)
	gw := httptest.NewServer(newProxy(up.URL, rec, nil, nil))
	defer gw.Close()

	// A realistic chat body so the measured overhead includes JSON parsing and
	// session resolution (anchor-hash), not just plumbing.
	const body = `{"model":"openai/gpt-4o-mini","user":"u-123","messages":[` +
		`{"role":"system","content":"You are a helpful assistant that answers concisely."},` +
		`{"role":"user","content":"What is the capital of France?"}],"stream":true}`

	const n = 300
	client := &http.Client{} // shared → keep-alive
	ttfts := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions", strings.NewReader(body))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		br := bufio.NewReader(resp.Body)
		if _, err := br.ReadBytes('\n'); err != nil { // first streamed line = time-to-first-token
			t.Fatal(err)
		}
		ttfts = append(ttfts, float64(time.Since(start).Microseconds())/1000.0)
		io.Copy(io.Discard, br)
		resp.Body.Close()
	}

	sort.Float64s(ttfts)
	p50 := ttfts[n*50/100]
	p99 := ttfts[n*99/100]
	snap := rec.Snapshot()
	t.Logf("end-to-end TTFT vs instant stub: p50=%.3fms p99=%.3fms (n=%d)", p50, p99, n)
	t.Logf("self-instrumented overhead:      p50=%.3fms p99=%.3fms max=%.3fms (n=%d)",
		snap.OverheadP50, snap.OverheadP99, snap.OverheadMax, snap.Count)

	if p50 > 5.0 {
		t.Errorf("p50 overhead %.3fms exceeds 5ms gate", p50)
	}
	if p99 > 15.0 {
		t.Errorf("p99 overhead %.3fms exceeds 15ms gate", p99)
	}
}

// stubUpstreamUsage streams a content chunk then a terminal usage frame carrying
// a reported cost — mimicking OpenRouter's in-stream cost reporting.
func stubUpstreamUsage(promptTok, compTok int, cost float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
		fl.Flush()
		fmt.Fprintf(w, "data: {\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"cost\":%v}}\n\n", promptTok, compTok, cost)
		fl.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		fl.Flush()
	}))
}

// TestMeteringEndToEnd drives a real meter through the proxy and asserts the
// session's recorded cost matches the provider-reported cost (the Phase 0 oracle,
// now an offline regression).
func TestMeteringEndToEnd(t *testing.T) {
	up := stubUpstreamUsage(14, 6, 0.0003)
	defer up.Close()
	store := meter.NewMemStore()
	mtr := meter.New(pricing.New(), store) // empty table → reported cost must be used
	gw := httptest.NewServer(newProxy(up.URL, nil, mtr, nil))
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions",
		strings.NewReader(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("X-Turnstile-Session", "s1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// metering is async — poll briefly.
	var rec meter.Record
	for i := 0; i < 50; i++ {
		if r, ok := store.Get("sess:s1"); ok {
			rec = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec.ID != "sess:s1" {
		t.Fatal("session not metered")
	}
	if rec.PromptTokens != 14 || rec.CompletionTokens != 6 {
		t.Errorf("tokens wrong: %+v", rec.Spend)
	}
	if rec.Cost != 0.0003 || rec.LastCost != string(pricing.SourceReported) {
		t.Errorf("cost reconciliation failed: cost=%v src=%q (want 0.0003/reported)", rec.Cost, rec.LastCost)
	}
}

// TestEnforcementBlocksWithProviderNativeError checks a killed session is blocked
// before forwarding, returned as a 429 provider-shaped error.
func TestEnforcementBlocksWithProviderNativeError(t *testing.T) {
	up := stubUpstream()
	defer up.Close()
	enf := enforce.New(enforce.Config{LoopEnabled: false}, meter.NewMemStore())
	enf.Kill("sess:doomed")
	gw := httptest.NewServer(newProxy(up.URL, nil, nil, enf))
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions",
		strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("X-Turnstile-Session", "doomed")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(b), "turnstile_blocked") || !strings.Contains(string(b), string(enforce.ReasonKilled)) {
		t.Fatalf("expected native block error, got %q", b)
	}
}

// failingEnforcer panics in Pre — used to prove the proxy fails open.
type failingEnforcer struct{}

func (failingEnforcer) Pre(session.Identity, []byte) enforce.Decision { panic("enforce boom") }
func (failingEnforcer) Killed(string) bool                            { return false }

func TestEnforcementFailsOpen(t *testing.T) {
	up := stubUpstream()
	defer up.Close()
	gw := httptest.NewServer(newProxy(up.URL, nil, nil, failingEnforcer{}))
	defer gw.Close()

	resp, err := http.Post(gw.URL+"/api/v1/chat/completions", "application/json",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "[DONE]") {
		t.Fatalf("panicking enforcer should fail open and forward, got %q", b)
	}
}

// stubUpstreamJSON returns a single non-streaming JSON body with a usage object
// (what a chat/completions call without stream=true gets).
func stubUpstreamJSON(promptTok, compTok int, cost float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"cost":%v}}`, promptTok, compTok, cost)
	}))
}

// TestMeteringNonStreaming proves a non-streaming JSON response is metered too
// (regression for the catalog-api case — plain chat/completions without streaming).
func TestMeteringNonStreaming(t *testing.T) {
	up := stubUpstreamJSON(40, 20, 0.0011)
	defer up.Close()
	store := meter.NewMemStore()
	mtr := meter.New(pricing.New(), store)
	gw := httptest.NewServer(newProxy(up.URL, nil, mtr, nil))
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions",
		strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("X-Turnstile-Session", "ns")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), `"content":"hi"`) {
		t.Fatalf("body not forwarded intact: %q", body)
	}

	var rec meter.Record
	for i := 0; i < 50; i++ {
		if r, ok := store.Get("sess:ns"); ok {
			rec = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec.ID != "sess:ns" {
		t.Fatal("non-streaming call was not metered")
	}
	if rec.PromptTokens != 40 || rec.CompletionTokens != 20 || rec.Cost != 0.0011 {
		t.Errorf("wrong metering: %+v", rec.Spend)
	}
}

// stubUpstreamJSONGzip returns a gzip-compressed non-streaming JSON body — what
// OpenRouter sends when the client offered Accept-Encoding: gzip.
func stubUpstreamJSONGzip(promptTok, compTok int, cost float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := fmt.Sprintf(`{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"cost":%v}}`, promptTok, compTok, cost)
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write([]byte(body))
		gz.Close()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(buf.Bytes())
	}))
}

// TestMeteringNonStreamingGzip is the catalog-api regression: a gzipped JSON
// response must still be metered (decompress a copy for parsing).
func TestMeteringNonStreamingGzip(t *testing.T) {
	up := stubUpstreamJSONGzip(40, 20, 0.0011)
	defer up.Close()
	store := meter.NewMemStore()
	mtr := meter.New(pricing.New(), store)
	gw := httptest.NewServer(newProxy(up.URL, nil, mtr, nil))
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions",
		strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("X-Turnstile-Session", "gz")
	resp, _ := http.DefaultClient.Do(req)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	var rec meter.Record
	for i := 0; i < 50; i++ {
		if r, ok := store.Get("sess:gz"); ok {
			rec = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec.ID != "sess:gz" {
		t.Fatal("gzipped non-streaming call was not metered")
	}
	if rec.PromptTokens != 40 || rec.CompletionTokens != 20 || rec.Cost != 0.0011 {
		t.Errorf("wrong metering: %+v", rec.Spend)
	}
}

// blockingStream emits one frame, then blocks until the test signals it to send
// more — long enough for the test to kill the session mid-stream and observe the
// live gate aborting the forward.
func blockingStream(release <-chan struct{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"chunk\":0}\n\n")
		fl.Flush()
		<-release
		fmt.Fprint(w, "data: {\"chunk\":1}\n\n")
		fl.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		fl.Flush()
	}))
}

// TestLiveGateAbortsKilledStream proves the mid-stream kill: once a session is
// killed while its response is streaming, the proxy stops forwarding further
// frames rather than running to [DONE].
func TestLiveGateAbortsKilledStream(t *testing.T) {
	release := make(chan struct{})
	up := blockingStream(release)
	defer up.Close()
	defer close(release)

	enf := enforce.New(enforce.Config{LoopEnabled: false}, meter.NewMemStore())
	gw := httptest.NewServer(newProxy(up.URL, nil, nil, enf))
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions",
		strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("X-Turnstile-Session", "live")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	br := bufio.NewReader(resp.Body)
	first, err := br.ReadBytes('\n')
	if err != nil || !bytes.Contains(first, []byte(`"chunk":0`)) {
		t.Fatalf("expected first frame before kill, got %q (err %v)", first, err)
	}

	// Kill mid-stream, then let the upstream try to send the rest.
	enf.Kill("sess:live")
	release <- struct{}{}

	rest, _ := io.ReadAll(br)
	if bytes.Contains(rest, []byte("[DONE]")) {
		t.Fatalf("killed stream should abort before [DONE], got %q", rest)
	}
}

// TestParseUsageIgnoresDoneAndContentFrames confirms only the usage frame yields
// usage: [DONE] and content deltas must not be metered (regression for $0/garbage
// rows from non-usage frames).
func TestParseUsageIgnoresDoneAndContentFrames(t *testing.T) {
	up := stubUpstreamUsage(3, 4, 0.002)
	defer up.Close()
	store := meter.NewMemStore()
	mtr := meter.New(pricing.New(), store)
	gw := httptest.NewServer(newProxy(up.URL, nil, mtr, nil))
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions",
		strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("X-Turnstile-Session", "usage")
	resp, _ := http.DefaultClient.Do(req)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	var rec meter.Record
	for i := 0; i < 50; i++ {
		if r, ok := store.Get("sess:usage"); ok {
			rec = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec.PromptTokens != 3 || rec.CompletionTokens != 4 || rec.Requests != 1 {
		t.Errorf("only the usage frame should meter once: %+v", rec.Spend)
	}
}

// stubUpstreamLargeJSON returns a non-streaming JSON body padded to ~padBytes
// with a junk field, carrying a valid usage object. Used to prove the metering
// buffer is bounded while the client still receives the complete body.
func stubUpstreamLargeJSON(padBytes int, promptTok, compTok int, cost float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pad := strings.Repeat("x", padBytes)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"pad":%q,"usage":{"prompt_tokens":%d,"completion_tokens":%d,"cost":%v}}`, pad, promptTok, compTok, cost)
	}))
}

// TestNonStreamingTransparencyAndBoundedMeter proves (a) the client receives the
// complete body byte-for-byte even when it far exceeds the metering cap, and (b)
// metering memory is bounded — within the cap usage still parses; over the cap
// the client body is still complete (transparency is non-negotiable).
func TestNonStreamingTransparencyAndBoundedMeter(t *testing.T) {
	// (a) Over-cap: body >> meter cap. Client must still get every byte; metering
	// may quietly fail (truncated buffer) but never truncates the response.
	t.Run("over_cap_full_body_to_client", func(t *testing.T) {
		const pad = 256 * 1024
		up := stubUpstreamLargeJSON(pad, 11, 7, 0.0009)
		defer up.Close()
		store := meter.NewMemStore()
		mtr := meter.New(pricing.New(), store)
		px := newProxy(up.URL, nil, mtr, nil)
		px.MaxMeterBytes = 4096 // tiny cap — body is far larger
		gw := httptest.NewServer(px)
		defer gw.Close()

		req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions",
			strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("X-Turnstile-Session", "big")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		want := fmt.Sprintf(`{"pad":%q,"usage":{"prompt_tokens":11,"completion_tokens":7,"cost":0.0009}}`, strings.Repeat("x", pad))
		if string(body) != want {
			t.Fatalf("client body not byte-for-byte complete: len got=%d want=%d", len(body), len(want))
		}
	})

	// (b) Within-cap: usage still parses through the size-limited buffer.
	t.Run("within_cap_meters", func(t *testing.T) {
		up := stubUpstreamLargeJSON(1024, 40, 20, 0.0011)
		defer up.Close()
		store := meter.NewMemStore()
		mtr := meter.New(pricing.New(), store)
		px := newProxy(up.URL, nil, mtr, nil)
		px.MaxMeterBytes = 8 << 20
		gw := httptest.NewServer(px)
		defer gw.Close()

		req, _ := http.NewRequest(http.MethodPost, gw.URL+"/api/v1/chat/completions",
			strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("X-Turnstile-Session", "cap")
		resp, _ := http.DefaultClient.Do(req)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		var rec meter.Record
		for i := 0; i < 50; i++ {
			if r, ok := store.Get("sess:cap"); ok {
				rec = r
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if rec.PromptTokens != 40 || rec.CompletionTokens != 20 || rec.Cost != 0.0011 {
			t.Errorf("within-cap metering failed: %+v", rec.Spend)
		}
	})
}

// panicSink panics on every Record — simulating a broken instrumentation path.
type panicSink struct{}

func (panicSink) Record(metrics.Sample) { panic("boom") }

// TestFailOpenOnSinkPanic verifies a panicking Sink cannot break the response
// (fail-open, Q3): instrumentation runs after the response and is recover-guarded.
func TestFailOpenOnSinkPanic(t *testing.T) {
	up := stubUpstream()
	defer up.Close()
	gw := httptest.NewServer(newProxy(up.URL, panicSink{}, nil, nil))
	defer gw.Close()

	resp, err := http.Post(gw.URL+"/api/v1/chat/completions", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "[DONE]") {
		t.Fatalf("response broken by sink panic: %q", b)
	}
	// give the async recorder goroutine a moment to run and recover
	time.Sleep(20 * time.Millisecond)
}
