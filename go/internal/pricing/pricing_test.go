package pricing

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"turnstile/internal/adapter"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

func TestReportedCostWins(t *testing.T) {
	e := New()
	e.SetTable(map[string]Price{"m": {Prompt: 1, Completion: 1}}) // would compute huge
	c, src := e.Cost("m", adapter.Usage{PromptTokens: 5, CompletionTokens: 5, ReportedCost: 0.42, HasReportedCost: true})
	if src != SourceReported || !approx(c, 0.42) {
		t.Fatalf("reported should win: got %v / %s", c, src)
	}
}

func TestOverrideBeatsTable(t *testing.T) {
	e := New()
	e.SetTable(map[string]Price{"m": {Prompt: 1, Completion: 1}})
	e.SetOverride(map[string]Price{"m": {Prompt: 0.5, Completion: 0.5}})
	c, src := e.Cost("m", adapter.Usage{PromptTokens: 10, CompletionTokens: 10})
	if src != SourceOverride || !approx(c, 10) { // 10*0.5 + 10*0.5
		t.Fatalf("override should win: got %v / %s", c, src)
	}
}

func TestTableFallback(t *testing.T) {
	e := New()
	e.SetTable(map[string]Price{"m": {Prompt: 0.001, Completion: 0.002}})
	c, src := e.Cost("m", adapter.Usage{PromptTokens: 100, CompletionTokens: 50})
	if src != SourceTable || !approx(c, 0.2) { // 0.1 + 0.1
		t.Fatalf("table fallback: got %v / %s", c, src)
	}
}

func TestUnknownModel(t *testing.T) {
	e := New()
	c, src := e.Cost("nope", adapter.Usage{PromptTokens: 10, CompletionTokens: 10})
	if src != SourceUnknown || c != 0 {
		t.Fatalf("unknown model: got %v / %s", c, src)
	}
}

func TestLoadOpenRouterTableSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"openai/gpt-4o-mini","pricing":{"prompt":"0.00000015","completion":"0.0000006"}}]}`)
	}))
	defer srv.Close()

	tbl, err := loadOpenRouterTable(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := tbl["openai/gpt-4o-mini"]
	if !ok || !approx(p.Prompt, 0.00000015) || !approx(p.Completion, 0.0000006) {
		t.Fatalf("parsed price wrong: %+v (ok=%v)", p, ok)
	}
}

func TestLoadOpenRouterTableContextCancelled(t *testing.T) {
	// Server blocks until the test ends; the cancelled context must abort the
	// request rather than hang.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	// Release before Close (defers are LIFO) so Close never blocks on the handler.
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	done := make(chan error, 1)
	go func() {
		_, err := loadOpenRouterTable(ctx, srv.URL)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error from cancelled context")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("load did not honour cancelled context")
	}
}

func TestParseTableJSON(t *testing.T) {
	tbl, err := ParseTableJSON([]byte(`{"m":{"Prompt":0.1,"Completion":0.2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if tbl["m"].Prompt != 0.1 || tbl["m"].Completion != 0.2 {
		t.Fatalf("parsed table wrong: %+v", tbl)
	}
}
