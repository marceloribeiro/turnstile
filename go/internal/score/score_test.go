package score

import (
	"strings"
	"testing"
)

func resultFor(t *testing.T, results []Result, name string) Result {
	t.Helper()
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no result named %q in %v", name, results)
	return Result{}
}

func TestEmptyFlagsBlankContent(t *testing.T) {
	for _, text := range []string{"", "   ", "\n\t "} {
		r := Empty{}.Score(Observation{Text: text})
		if r.Pass {
			t.Errorf("Empty passed on %q, want fail", text)
		}
		if r.Detail != "no_content" {
			t.Errorf("detail = %q, want no_content", r.Detail)
		}
	}
	if r := (Empty{}).Score(Observation{Text: "hello"}); !r.Pass {
		t.Error("Empty failed on real content")
	}
}

func TestTruncatedAcceptsEachProviderSpelling(t *testing.T) {
	// Providers disagree on the wording; all of them mean "ran out of room".
	for _, reason := range []string{"length", "max_tokens", "MAX_TOKENS", " Length "} {
		if r := (Truncated{}).Score(Observation{FinishReason: reason}); r.Pass {
			t.Errorf("Truncated passed on finish reason %q, want fail", reason)
		}
	}
	for _, reason := range []string{"stop", "end_turn", ""} {
		if r := (Truncated{}).Score(Observation{FinishReason: reason}); !r.Pass {
			t.Errorf("Truncated failed on finish reason %q, want pass", reason)
		}
	}
}

func TestJSONValidOnlyJudgesStructuredWorkloads(t *testing.T) {
	// Without ExpectJSON the scorer must not judge prose, so it is safe to run
	// across all traffic.
	if r := (JSONValid{}).Score(Observation{Text: "just prose"}); !r.Pass {
		t.Error("JSONValid judged a non-structured response")
	}

	ok := (JSONValid{}).Score(Observation{Text: `{"a":1}`, ExpectJSON: true})
	if !ok.Pass {
		t.Error("JSONValid failed on valid JSON")
	}

	bad := (JSONValid{}).Score(Observation{Text: "Sure! Here you go.", ExpectJSON: true})
	if bad.Pass || bad.Detail != "invalid" {
		t.Errorf("prose: pass=%v detail=%q, want fail/invalid", bad.Pass, bad.Detail)
	}

	empty := (JSONValid{}).Score(Observation{Text: "  ", ExpectJSON: true})
	if empty.Pass || empty.Detail != "no_content" {
		t.Errorf("empty: pass=%v detail=%q, want fail/no_content", empty.Pass, empty.Detail)
	}
}

func TestJSONValidDistinguishesFencedJSON(t *testing.T) {
	// A fenced object is the right answer wrapped wrong — a prompt problem, not
	// a comprehension one — so it gets its own detail.
	for _, text := range []string{
		"```json\n{\"a\":1}\n```",
		"```\n{\"a\":1}\n```",
	} {
		r := (JSONValid{}).Score(Observation{Text: text, ExpectJSON: true})
		if r.Pass {
			t.Errorf("fenced JSON passed for %q; it is not parseable as returned", text)
		}
		if r.Detail != "fenced" {
			t.Errorf("detail = %q for %q, want fenced", r.Detail, text)
		}
	}
}

func TestRefusalMatchesOpeningOnly(t *testing.T) {
	declined := (Refusal{}).Score(Observation{
		Text: "I'm sorry, but I can't help with that request.",
	})
	if declined.Pass {
		t.Error("Refusal passed on a declined response")
	}

	// The same phrase deep inside a long answer is the model *discussing*
	// refusals, not performing one. Anchoring to the opening avoids that.
	discussion := (Refusal{}).Score(Observation{
		Text: strings.Repeat("Here is a detailed explanation of the topic. ", 20) +
			"A model might say I'm sorry, but I can't help with that.",
	})
	if !discussion.Pass {
		t.Error("Refusal fired on a phrase far from the opening")
	}

	if r := (Refusal{}).Score(Observation{Text: "Certainly — here's how."}); !r.Pass {
		t.Error("Refusal fired on a compliant response")
	}
}

func TestBudgetScorersSkipWhenUnset(t *testing.T) {
	// No budget means nothing to judge; these must pass rather than fail open
	// into noise.
	lat := (LatencyBudget{}).Score(Observation{TotalMs: 9999})
	if !lat.Pass || lat.Detail != "no_budget" {
		t.Errorf("latency without budget: pass=%v detail=%q", lat.Pass, lat.Detail)
	}
	cost := (CostBudget{}).Score(Observation{CostUSD: 100})
	if !cost.Pass || cost.Detail != "no_budget" {
		t.Errorf("cost without budget: pass=%v detail=%q", cost.Pass, cost.Detail)
	}
}

func TestBudgetScorersFlagBreaches(t *testing.T) {
	lat := (LatencyBudget{}).Score(Observation{TotalMs: 2500, LatencyBudgetMs: 2000})
	if lat.Pass || lat.Value != 2500 {
		t.Errorf("latency breach: pass=%v value=%v", lat.Pass, lat.Value)
	}
	within := (LatencyBudget{}).Score(Observation{TotalMs: 1500, LatencyBudgetMs: 2000})
	if !within.Pass {
		t.Error("latency within budget was flagged")
	}

	cost := (CostBudget{}).Score(Observation{CostUSD: 0.5, CostBudgetUSD: 0.25})
	if cost.Pass {
		t.Error("cost breach not flagged")
	}
}

func TestSetRunsEveryScorerInOrder(t *testing.T) {
	set := NewSet()
	results := set.Run(Observation{Text: "hello", FinishReason: "stop"})
	if len(results) != len(Default()) {
		t.Fatalf("got %d results, want %d", len(results), len(Default()))
	}
	for i, sc := range Default() {
		if results[i].Name != sc.Name() {
			t.Errorf("result %d is %q, want %q", i, results[i].Name, sc.Name())
		}
	}
}

type panickingScorer struct{}

func (panickingScorer) Name() string { return "boom" }
func (panickingScorer) Score(Observation) Result {
	panic("scorer exploded")
}

func TestPanickingScorerIsContained(t *testing.T) {
	// Scoring is instrumentation: a broken scorer must degrade to "unscored",
	// never take down the batch or the request path.
	set := &Set{Scorers: []Scorer{panickingScorer{}, Empty{}}}
	results := set.Run(Observation{Text: "hello"})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 — the panic ate a scorer", len(results))
	}
	boom := resultFor(t, results, "boom")
	if boom.Pass || boom.Detail != "scorer_panic" {
		t.Errorf("panicking scorer: pass=%v detail=%q", boom.Pass, boom.Detail)
	}
	if !resultFor(t, results, "empty").Pass {
		t.Error("the scorer after the panicking one did not run")
	}
}

func TestSummarizeCountsAndNamesFailures(t *testing.T) {
	o := Observation{
		SessionID:    "sess-1",
		Adapter:      "openai",
		Model:        "gpt-x",
		Text:         "",
		FinishReason: "length",
	}
	sum := Summarize(o, NewSet().Run(o))

	if sum.SessionID != "sess-1" || sum.Adapter != "openai" || sum.Model != "gpt-x" {
		t.Errorf("summary lost identity: %+v", sum)
	}
	if sum.Total != len(Default()) {
		t.Errorf("Total = %d, want %d", sum.Total, len(Default()))
	}
	// Empty content and a truncated finish reason are both real failures here.
	for _, want := range []string{"empty", "truncated"} {
		found := false
		for _, f := range sum.Failed {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q among failures, got %v", want, sum.Failed)
		}
	}
	if sum.Passed+len(sum.Failed) != sum.Total {
		t.Errorf("passed(%d) + failed(%d) != total(%d)", sum.Passed, len(sum.Failed), sum.Total)
	}
}

func TestPassRate(t *testing.T) {
	if got := (Summary{}).PassRate(); got != 1 {
		t.Errorf("empty summary PassRate = %v, want 1", got)
	}
	if got := (Summary{Total: 4, Passed: 3}).PassRate(); got != 0.75 {
		t.Errorf("PassRate = %v, want 0.75", got)
	}
}

func TestSummaryCarriesNoResponseContent(t *testing.T) {
	// The privacy commitment in code: a Summary is numbers and labels. If this
	// test ever fails, content is leaking to telemetry.
	secret := "PATIENT NAME: Jane Doe, diagnosis withheld"
	o := Observation{SessionID: "s", Text: secret, ExpectJSON: true}
	sum := Summarize(o, NewSet().Run(o))

	for _, r := range sum.Results {
		if strings.Contains(r.Detail, secret) || strings.Contains(r.Name, secret) {
			t.Fatalf("scorer result leaked response content: %+v", r)
		}
	}
	for _, f := range sum.Failed {
		if strings.Contains(f, secret) {
			t.Fatalf("failure list leaked response content: %v", sum.Failed)
		}
	}
}
