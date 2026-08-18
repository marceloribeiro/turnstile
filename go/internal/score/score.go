// Package score evaluates completed LLM responses out-of-band.
//
// Scoring runs after the response has been forwarded, on the same discipline as
// metrics: it can never add latency to, or break, the request path. Scorers are
// deterministic and stdlib-only; anything needing a model call belongs in the
// control plane (see docs/EVALS.md).
//
// Content stays in memory. A Result carries numbers and labels — never prompt
// or completion text — so scoring preserves the guarantee that raw content
// never leaves the data plane.
package score

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Observation is one completed exchange, as much as scoring needs to judge it.
// Text is held only for the duration of the call.
type Observation struct {
	SessionID    string
	Adapter      string
	Model        string
	Text         string  // assistant content, joined across frames
	FinishReason string  // "stop" | "length" | "content_filter" | provider-specific
	TotalMs      float64 // request received -> stream complete
	CostUSD      float64 // metered cost of this exchange
	// Budgets are per-session ceilings; zero means "no budget, skip the check".
	LatencyBudgetMs float64
	CostBudgetUSD   float64
	// ExpectJSON marks a structured-output workload, so prose is a failure.
	ExpectJSON bool
}

// Result is one scorer's verdict. Value is the measured quantity (a ratio, a
// count, milliseconds, dollars); Pass is the verdict against it.
type Result struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Pass  bool    `json:"pass"`
	// Detail is a short machine-readable reason, never response content.
	Detail string `json:"detail,omitempty"`
}

// Scorer judges one Observation. Implementations must be pure and fast: they
// run on every request, and a slow scorer steals capacity even off the request
// path.
type Scorer interface {
	Name() string
	Score(Observation) Result
}

// refusalMarkers are lowercase phrases that open a declined response. Matching
// is restricted to the opening of the text: a refusal is a response that starts
// by declining, whereas the same phrase deep in a long answer is usually the
// model discussing refusals rather than performing one.
var refusalMarkers = []string{
	"i'm sorry, but i can't",
	"i'm sorry, but i cannot",
	"i am sorry, but i cannot",
	"i can't help with that",
	"i cannot help with that",
	"i can't assist with that",
	"i cannot assist with that",
	"i'm unable to help",
	"i am unable to help",
	"i won't be able to help",
	"as an ai language model, i can't",
	"as an ai language model, i cannot",
}

// refusalPrefixLen bounds how much of the response counts as "the opening".
const refusalPrefixLen = 240

// Empty flags a response with no usable content. The commonest silent failure:
// the call succeeded, billed tokens, and returned nothing.
type Empty struct{}

func (Empty) Name() string { return "empty" }

func (Empty) Score(o Observation) Result {
	n := len(strings.TrimSpace(o.Text))
	return Result{Name: "empty", Value: float64(n), Pass: n > 0, Detail: detailIf(n == 0, "no_content")}
}

// Truncated flags a response cut off at the token ceiling. The answer may read
// fine and still be missing its ending — for structured output it is usually
// unparseable.
type Truncated struct{}

func (Truncated) Name() string { return "truncated" }

func (Truncated) Score(o Observation) Result {
	cut := isTruncatedReason(o.FinishReason)
	v := 0.0
	if cut {
		v = 1
	}
	return Result{Name: "truncated", Value: v, Pass: !cut, Detail: detailIf(cut, o.FinishReason)}
}

// isTruncatedReason normalises the providers' several spellings of "ran out of
// room": OpenAI/OpenRouter use "length", Anthropic "max_tokens", Gemini
// "MAX_TOKENS".
func isTruncatedReason(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "length", "max_tokens", "maxtokens":
		return true
	}
	return false
}

// JSONValid flags structured-output workloads that came back as prose. Only
// meaningful when the caller marks the observation with ExpectJSON; otherwise
// it passes without judging, so it is safe to run over all traffic.
type JSONValid struct{}

func (JSONValid) Name() string { return "json_valid" }

func (JSONValid) Score(o Observation) Result {
	if !o.ExpectJSON {
		return Result{Name: "json_valid", Value: 1, Pass: true, Detail: "not_applicable"}
	}
	text := strings.TrimSpace(o.Text)
	if text == "" {
		return Result{Name: "json_valid", Value: 0, Pass: false, Detail: "no_content"}
	}
	if json.Valid([]byte(text)) {
		return Result{Name: "json_valid", Value: 1, Pass: true}
	}
	// Fenced JSON is a near-miss worth distinguishing from prose: the model
	// produced the right object and wrapped it, which is a prompt problem
	// rather than a comprehension one.
	if inner, ok := unfence(text); ok && json.Valid([]byte(inner)) {
		return Result{Name: "json_valid", Value: 0, Pass: false, Detail: "fenced"}
	}
	return Result{Name: "json_valid", Value: 0, Pass: false, Detail: "invalid"}
}

// unfence strips a leading ```lang / trailing ``` markdown fence, returning the
// content between them.
func unfence(s string) (string, bool) {
	if !strings.HasPrefix(s, "```") {
		return "", false
	}
	rest := s[3:]
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		lang := strings.TrimSpace(rest[:i])
		// A fence opener is ``` or ```json — anything longer is not a language tag.
		if strings.ContainsFunc(lang, func(r rune) bool { return unicode.IsSpace(r) }) {
			return "", false
		}
		rest = rest[i+1:]
	}
	if j := strings.LastIndex(rest, "```"); j >= 0 {
		return strings.TrimSpace(rest[:j]), true
	}
	return "", false
}

// Refusal flags a response that opens by declining the task. A refusal is not
// inherently a defect — it is the correct answer to some requests — but a rate
// that moves is one of the clearest signals that a prompt or model change has
// regressed.
type Refusal struct{}

func (Refusal) Name() string { return "refusal" }

func (Refusal) Score(o Observation) Result {
	head := strings.ToLower(strings.TrimSpace(o.Text))
	if len(head) > refusalPrefixLen {
		head = head[:refusalPrefixLen]
	}
	for _, m := range refusalMarkers {
		if strings.Contains(head, m) {
			return Result{Name: "refusal", Value: 1, Pass: false, Detail: "declined"}
		}
	}
	return Result{Name: "refusal", Value: 0, Pass: true}
}

// LatencyBudget flags a response slower than the session's ceiling. Passes when
// no budget is set, so it is safe to run over all traffic.
type LatencyBudget struct{}

func (LatencyBudget) Name() string { return "latency_budget" }

func (LatencyBudget) Score(o Observation) Result {
	if o.LatencyBudgetMs <= 0 {
		return Result{Name: "latency_budget", Value: o.TotalMs, Pass: true, Detail: "no_budget"}
	}
	over := o.TotalMs > o.LatencyBudgetMs
	return Result{Name: "latency_budget", Value: o.TotalMs, Pass: !over, Detail: detailIf(over, "over_budget")}
}

// CostBudget flags a single exchange more expensive than the session's ceiling.
// Distinct from the enforcement budget, which governs cumulative session spend:
// this catches one pathologically expensive call inside an otherwise cheap
// session.
type CostBudget struct{}

func (CostBudget) Name() string { return "cost_budget" }

func (CostBudget) Score(o Observation) Result {
	if o.CostBudgetUSD <= 0 {
		return Result{Name: "cost_budget", Value: o.CostUSD, Pass: true, Detail: "no_budget"}
	}
	over := o.CostUSD > o.CostBudgetUSD
	return Result{Name: "cost_budget", Value: o.CostUSD, Pass: !over, Detail: detailIf(over, "over_budget")}
}

func detailIf(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}

// Default returns the structural scorers, in reporting order. They are
// deterministic, dependency-free, and cheap enough to run on every request.
func Default() []Scorer {
	return []Scorer{Empty{}, Truncated{}, JSONValid{}, Refusal{}, LatencyBudget{}, CostBudget{}}
}

// Set runs a group of scorers over one Observation. A panicking scorer is
// contained and reported as an unscored failure rather than taking down the
// batch — scoring is instrumentation, and instrumentation must not break the
// thing it observes.
type Set struct {
	Scorers []Scorer
}

// NewSet builds a Set from the default scorers.
func NewSet() *Set { return &Set{Scorers: Default()} }

// Run scores one Observation, returning one Result per scorer in order.
func (s *Set) Run(o Observation) []Result {
	if s == nil || len(s.Scorers) == 0 {
		return nil
	}
	out := make([]Result, 0, len(s.Scorers))
	for _, sc := range s.Scorers {
		out = append(out, safeScore(sc, o))
	}
	return out
}

// safeScore isolates one scorer so a panic becomes a failed Result.
func safeScore(sc Scorer, o Observation) (r Result) {
	defer func() {
		if rec := recover(); rec != nil {
			r = Result{Name: sc.Name(), Value: 0, Pass: false, Detail: "scorer_panic"}
		}
	}()
	return sc.Score(o)
}

// Summary reduces a run to the numbers the dashboard and telemetry carry: how
// many scorers ran, how many passed, and which failed. No content, by design.
type Summary struct {
	SessionID string   `json:"session_id"`
	Adapter   string   `json:"adapter,omitempty"`
	Model     string   `json:"model,omitempty"`
	Total     int      `json:"total"`
	Passed    int      `json:"passed"`
	Failed    []string `json:"failed,omitempty"`
	Results   []Result `json:"results,omitempty"`
}

// Summarize folds results into a Summary for one observation.
func Summarize(o Observation, results []Result) Summary {
	s := Summary{
		SessionID: o.SessionID,
		Adapter:   o.Adapter,
		Model:     o.Model,
		Total:     len(results),
		Results:   results,
	}
	for _, r := range results {
		if r.Pass {
			s.Passed++
			continue
		}
		s.Failed = append(s.Failed, r.Name)
	}
	return s
}

// PassRate is the share of scorers that passed, in [0,1]. An empty run scores 1
// — nothing was judged, so nothing failed.
func (s Summary) PassRate() float64 {
	if s.Total == 0 {
		return 1
	}
	return float64(s.Passed) / float64(s.Total)
}
