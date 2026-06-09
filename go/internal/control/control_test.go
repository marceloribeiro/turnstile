package control

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"turnstile/internal/events"
	"turnstile/internal/meter"
	"turnstile/internal/session"
)

// fakeKiller records kill/unkill calls.
type fakeKiller struct{ killed map[string]bool }

func newKiller() *fakeKiller                { return &fakeKiller{killed: map[string]bool{}} }
func (f *fakeKiller) Kill(id string)        { f.killed[id] = true }
func (f *fakeKiller) Unkill(id string)      { delete(f.killed, id) }
func (f *fakeKiller) Killed(id string) bool { return f.killed[id] }

func seedStore() *meter.MemStore {
	st := meter.NewMemStore()
	st.Add(session.Identity{ID: "sess:a", Source: session.SourceHeader}, "m",
		meter.Spend{Requests: 2, Cost: 0.5}, "reported")
	st.RecordPrevented("sess:a", 0.1)
	return st
}

func newServer(token string) (*Server, *fakeKiller) {
	k := newKiller()
	return New(seedStore(), k, events.NewBroker(), token, "*"), k
}

func TestListSessions(t *testing.T) {
	s, _ := newServer("")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/v1/sessions")
	var got []meter.Record
	json.NewDecoder(resp.Body).Decode(&got)
	if len(got) != 1 || got[0].ID != "sess:a" || got[0].Cost != 0.5 {
		t.Fatalf("unexpected sessions: %+v", got)
	}
}

func TestSummaryAggregates(t *testing.T) {
	s, _ := newServer("")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/v1/summary")
	var got map[string]any
	json.NewDecoder(resp.Body).Decode(&got)
	if got["dollars_prevented"].(float64) != 0.1 || got["total_cost"].(float64) != 0.5 {
		t.Fatalf("summary wrong: %+v", got)
	}
}

func TestKillReachesEnforcer(t *testing.T) {
	s, k := newServer("")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/sessions/sess:a/kill", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("kill status %d", resp.StatusCode)
	}
	if !k.Killed("sess:a") {
		t.Fatal("enforcer was not asked to kill the session")
	}
}

func TestAuthRequiredWhenTokenSet(t *testing.T) {
	s, _ := newServer("secret")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// no token → 401
	resp, _ := http.Get(srv.URL + "/v1/sessions")
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
	// healthz stays open
	h, _ := http.Get(srv.URL + "/v1/healthz")
	if h.StatusCode != 200 {
		t.Fatalf("healthz should be unauthenticated, got %d", h.StatusCode)
	}
	// with token → 200
	req, _ := http.NewRequest("GET", srv.URL+"/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	ok, _ := http.DefaultClient.Do(req)
	if ok.StatusCode != 200 {
		t.Fatalf("expected 200 with token, got %d", ok.StatusCode)
	}
}

// TestAuthRejectsWrongTokens covers the constant-time compare: wrong values and
// near-misses (correct length, off by one char; correct prefix, wrong length)
// must all be rejected, while the exact token is accepted.
func TestAuthRejectsWrongTokens(t *testing.T) {
	s, _ := newServer("secret")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	cases := []struct {
		header string
		want   int
	}{
		{"", 401},
		{"Bearer wrong!", 401},       // same length, different bytes
		{"Bearer secre", 401},        // shorter
		{"Bearer secret-extra", 401}, // longer
		{"secret", 401},              // missing scheme
		{"Bearer secret", 200},       // exact match
	}
	for _, c := range cases {
		req, _ := http.NewRequest("GET", srv.URL+"/v1/sessions", nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("header %q: status %d, want %d", c.header, resp.StatusCode, c.want)
		}
	}
}

func TestCORSPreflight(t *testing.T) {
	s, _ := newServer("")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	req, _ := http.NewRequest("OPTIONS", srv.URL+"/v1/sessions", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS header")
	}
}

func TestEventsSSEStream(t *testing.T) {
	k := newKiller()
	broker := events.NewBroker()
	s := New(seedStore(), k, broker, "", "*")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// publish after the subscriber is connected
	go func() {
		for i := 0; i < 20; i++ {
			broker.Publish(events.Event{Type: "blocked", SessionID: "sX", Reason: "loop_detected"})
		}
	}()

	buf := make([]byte, 4096)
	deadlineRead := make(chan string, 1)
	go func() {
		acc := ""
		for {
			n, err := resp.Body.Read(buf)
			acc += string(buf[:n])
			if strings.Contains(acc, "loop_detected") || err != nil {
				deadlineRead <- acc
				return
			}
		}
	}()
	got := <-deadlineRead
	if !strings.Contains(got, "data:") || !strings.Contains(got, "loop_detected") {
		t.Fatalf("SSE stream missing event, got: %q", got)
	}
	_ = io.Discard
}
