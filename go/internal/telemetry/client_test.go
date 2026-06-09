package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"turnstile/internal/meter"
)

type fakeSource struct{ recs []meter.Record }

func (f fakeSource) Sessions() []meter.Record { return f.recs }

func sample() []meter.Record {
	r := meter.Record{ID: "sess:vip", Source: "header", User: "u1", Model: "m"}
	r.Requests = 3
	r.Cost = 0.0021
	r.Blocks = 1
	r.Prevented = 0.0007
	return []meter.Record{r}
}

func TestPushSendsGoShapedPayloadWithKey(t *testing.T) {
	var (
		mu     sync.Mutex
		gotKey string
		gotErr error
		body   ingestBody
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotKey = r.Header.Get("X-Turnstile-Ingest-Key")
		b, _ := io.ReadAll(r.Body)
		gotErr = json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"upserted":1}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "ts_testkey", time.Minute, fakeSource{recs: sample()})
	c.push(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if gotKey != "ts_testkey" {
		t.Errorf("ingest key header = %q", gotKey)
	}
	if gotErr != nil {
		t.Fatalf("server could not parse payload: %v", gotErr)
	}
	if len(body.Sessions) != 1 || body.Sessions[0].ID != "sess:vip" || body.Sessions[0].Cost != 0.0021 {
		t.Fatalf("unexpected payload: %+v", body.Sessions)
	}
}

func TestPushSkipsWhenEmpty(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	New(srv.URL, "k", time.Minute, fakeSource{recs: nil}).push(context.Background())
	if called {
		t.Error("push should not POST when there are no sessions")
	}
}

func TestPushFailsOpenOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	// Must not panic; just logs and drops.
	New(srv.URL, "k", time.Minute, fakeSource{recs: sample()}).push(context.Background())
}

func TestPushFailsOpenOnUnreachable(t *testing.T) {
	// Nothing listening — must not panic.
	New("http://127.0.0.1:1/ingest/sessions", "k", time.Minute, fakeSource{recs: sample()}).push(context.Background())
}

// TestPushHonoursCancelledContext proves a cancelled context aborts an in-flight
// POST promptly rather than blocking on a slow ingest endpoint.
func TestPushHonoursCancelledContext(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // hold the connection open until the test releases it
	}))
	// Release the handler before closing the server so Close doesn't block on the
	// in-flight request (defers run LIFO — this runs before srv.Close).
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	c := New(srv.URL, "k", time.Minute, fakeSource{recs: sample()})

	done := make(chan struct{})
	go func() {
		c.push(ctx) // must return once the context is cancelled
		close(done)
	}()

	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("push did not abort on context cancel")
	}
}
