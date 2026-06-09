package meter

import (
	"math"
	"testing"

	"turnstile/internal/adapter"
	"turnstile/internal/pricing"
	"turnstile/internal/session"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func ident(id string) session.Identity {
	return session.Identity{ID: id, Source: session.SourceHeader, User: "u", KeyFP: "fp"}
}

func TestObserveAccumulatesFromTable(t *testing.T) {
	e := pricing.New()
	e.SetTable(map[string]pricing.Price{"m": {Prompt: 0.001, Completion: 0.002}})
	st := NewMemStore()
	m := New(e, st)

	m.Observe(ident("sess:1"), "m", adapter.Usage{PromptTokens: 100, CompletionTokens: 50})
	m.Observe(ident("sess:1"), "m", adapter.Usage{PromptTokens: 10, CompletionTokens: 5})

	rec, ok := st.Get("sess:1")
	if !ok {
		t.Fatal("session not recorded")
	}
	if rec.Requests != 2 || rec.PromptTokens != 110 || rec.CompletionTokens != 55 {
		t.Errorf("token accumulation wrong: %+v", rec.Spend)
	}
	if !approx(rec.Cost, 0.22) { // (0.1+0.1)+(0.01+0.01)
		t.Errorf("cost = %v, want 0.22", rec.Cost)
	}
	if rec.LastCost != string(pricing.SourceTable) {
		t.Errorf("LastCost = %q", rec.LastCost)
	}
}

func TestObservePrefersReportedCost(t *testing.T) {
	st := NewMemStore()
	m := New(pricing.New(), st) // empty table — only reported cost can price it
	m.Observe(ident("sess:2"), "m", adapter.Usage{PromptTokens: 1, CompletionTokens: 1, ReportedCost: 0.005, HasReportedCost: true})

	rec, _ := st.Get("sess:2")
	if !approx(rec.Cost, 0.005) || rec.LastCost != string(pricing.SourceReported) {
		t.Errorf("reported cost not used: %+v", rec)
	}
}

func TestSessionsSortedByCostDesc(t *testing.T) {
	e := pricing.New()
	st := NewMemStore()
	m := New(e, st)
	m.Observe(ident("cheap"), "m", adapter.Usage{ReportedCost: 0.01, HasReportedCost: true})
	m.Observe(ident("pricey"), "m", adapter.Usage{ReportedCost: 9.0, HasReportedCost: true})
	got := st.Sessions()
	if len(got) != 2 || got[0].ID != "pricey" {
		t.Errorf("expected pricey first, got %+v", got)
	}
}
