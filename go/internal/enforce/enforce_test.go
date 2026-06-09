package enforce

import (
	"fmt"
	"testing"
	"time"

	"turnstile/internal/session"
)

// fakeLedger is a minimal Ledger for tests.
type fakeLedger struct {
	cost      float64
	requests  int64
	prevented float64
}

func (f *fakeLedger) SessionSpend(id string) (float64, int64) { return f.cost, f.requests }
func (f *fakeLedger) RecordPrevented(id string, amount float64) {
	f.prevented += amount
}

func ident(id string) session.Identity { return session.Identity{ID: id, Source: session.SourceHeader} }

func cfg() Config {
	return Config{LoopEnabled: true, LoopThreshold: 5, LoopWindow: time.Minute}
}

func TestAllowsByDefault(t *testing.T) {
	e := New(cfg(), &fakeLedger{})
	if d := e.Pre(ident("s"), []byte(`{"a":1}`)); !d.Allow {
		t.Fatalf("expected allow, got %+v", d)
	}
}

func TestManualKillBlocks(t *testing.T) {
	led := &fakeLedger{cost: 0.10, requests: 5}
	e := New(cfg(), led)
	e.Kill("s")
	d := e.Pre(ident("s"), []byte(`{"a":1}`))
	if d.Allow || d.Reason != ReasonKilled {
		t.Fatalf("expected killed block, got %+v", d)
	}
	if led.prevented != 0.02 { // avg cost/req = 0.10/5
		t.Errorf("dollars prevented = %v, want 0.02", led.prevented)
	}
	e.Unkill("s")
	if d := e.Pre(ident("s"), []byte(`{"a":1}`)); !d.Allow {
		t.Errorf("unkill should restore allow")
	}
}

func TestBudgetCeilingBlocks(t *testing.T) {
	c := cfg()
	c.SessionBudgetUSD = 1.0
	led := &fakeLedger{cost: 1.5, requests: 3}
	e := New(c, led)
	d := e.Pre(ident("s"), []byte(`{"a":1}`))
	if d.Allow || d.Reason != ReasonBudget {
		t.Fatalf("expected budget block, got %+v", d)
	}
}

func TestBudgetDisabledWhenZero(t *testing.T) {
	led := &fakeLedger{cost: 9999, requests: 1}
	e := New(cfg(), led) // budget 0 = disabled
	if d := e.Pre(ident("s"), []byte(`{"a":1}`)); !d.Allow {
		t.Errorf("budget 0 should not block, got %+v", d)
	}
}

func TestLoopTripsAtThreshold(t *testing.T) {
	e := New(cfg(), &fakeLedger{}) // threshold 5
	body := []byte(`{"same":"call"}`)
	for i := 1; i <= 4; i++ {
		if d := e.Pre(ident("s"), body); !d.Allow {
			t.Fatalf("call %d should be allowed", i)
		}
	}
	if d := e.Pre(ident("s"), body); d.Allow || d.Reason != ReasonLoop {
		t.Fatalf("5th identical call should trip loop, got %+v", d)
	}
}

func TestLoopDoesNotTripOnVaryingBodies(t *testing.T) {
	e := New(cfg(), &fakeLedger{})
	for i := 0; i < 20; i++ {
		body := []byte(fmt.Sprintf(`{"turn":%d}`, i)) // growing/varying = normal convo
		if d := e.Pre(ident("s"), body); !d.Allow {
			t.Fatalf("varying body %d should not trip, got %+v", i, d)
		}
	}
}

// loopKeyCount reports how many windows the enforcer is tracking (test-only view).
func loopKeyCount(e *Enforcer) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.loops)
}

func TestLoopMapEvictsExpiredKeys(t *testing.T) {
	c := Config{LoopEnabled: true, LoopThreshold: 100, LoopWindow: 20 * time.Millisecond}
	e := New(c, &fakeLedger{})

	// Seed many distinct keys (distinct bodies → distinct fingerprints).
	for i := 0; i < 50; i++ {
		e.Pre(ident("s"), []byte(fmt.Sprintf(`{"k":%d}`, i)))
	}
	if got := loopKeyCount(e); got != 50 {
		t.Fatalf("expected 50 tracked keys, got %d", got)
	}

	// Let every window expire, then a sweep (triggered by a fresh call after a
	// full window has elapsed) must remove the stale keys.
	time.Sleep(40 * time.Millisecond)
	e.Pre(ident("s"), []byte(`{"fresh":1}`))
	if got := loopKeyCount(e); got != 1 {
		t.Fatalf("expected stale keys evicted (1 fresh key left), got %d", got)
	}
}

func TestLoopStillTripsAfterEviction(t *testing.T) {
	c := Config{LoopEnabled: true, LoopThreshold: 3, LoopWindow: time.Minute}
	e := New(c, &fakeLedger{})
	// Churn unrelated keys to drive a sweep, then confirm detection still trips.
	for i := 0; i < 10; i++ {
		e.Pre(ident("noise"), []byte(fmt.Sprintf(`{"n":%d}`, i)))
	}
	body := []byte(`{"same":"call"}`)
	if d := e.Pre(ident("s"), body); !d.Allow {
		t.Fatal("call 1 should be allowed")
	}
	if d := e.Pre(ident("s"), body); !d.Allow {
		t.Fatal("call 2 should be allowed")
	}
	if d := e.Pre(ident("s"), body); d.Allow || d.Reason != ReasonLoop {
		t.Fatalf("3rd identical call should trip loop, got %+v", d)
	}
}

func TestLoopScopedPerSession(t *testing.T) {
	e := New(cfg(), &fakeLedger{})
	body := []byte(`{"x":1}`)
	for i := 0; i < 10; i++ {
		e.Pre(ident("sessionA"), body)
	}
	// A different session with the same body must be unaffected.
	if d := e.Pre(ident("sessionB"), body); !d.Allow {
		t.Errorf("loop in session A must not block session B")
	}
}
