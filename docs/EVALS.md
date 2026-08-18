# Evals at the proxy layer

> Status: design + structural scorers landed (`go/internal/score`). Adapter
> content extraction and control-plane judging are the next steps.

Turnstile already sees every request and response that passes between an
application and its LLM provider. That is a privileged position for evaluation:
it can score **production traffic** without the application being instrumented
at all.

Every other eval tool asks you to change your code — wrap calls, add a tracing
SDK, or run offline against a dataset you had to build yourself. Turnstile is
already in the path. Swapping one base URL is the entire integration.

## Why this belongs in Turnstile

Turnstile trips a breaker when a session goes **expensive** (budget ceiling) or
**looped** (loop detection). Those are proxies for "something has gone wrong".
The direct signal is quality, and quality is what evals measure.

The end state is a breaker that fires on a *quality* regression: kill the
session that has gone wrong, not just the one that has gone costly. Cost and
loop detection stay as they are; scoring adds the third and most direct signal.

## Constraints this design must respect

These are existing product commitments, not preferences:

1. **The latency gate is sacred.** Scoring never runs on the request path. It
   observes asynchronously through the same `metrics.Sink` discipline the
   recorder and event broker already use — dispatched after the response is
   complete, panic-guarded, and incapable of adding time-to-first-token.
2. **Raw prompts and completions are never persisted.** Scoring happens
   in-process on bytes already in memory; only numeric results and boolean
   labels leave the data plane. A score is an aggregate, not content.
3. **Fail-open.** A scorer that panics, hangs or errors must never affect the
   proxied request. Scoring failures degrade to "unscored", never to an error.
4. **Stdlib-only in the data plane.** Structural scorers add no dependencies.
   Anything needing a model call belongs in the control plane.

Constraint 2 is the reason the eval layer is split across the two planes:
deterministic scoring runs where the bytes already are; model-based judging
runs where an LLM call is normal, on content the operator has explicitly opted
to retain.

## Architecture

```
request ──► Go data plane ──► provider
                 │
                 ├─ forward bytes (untouched, first)
                 ├─ parse usage        (existing, off-path)
                 └─ score              (new, off-path)
                        │
                        ▼
                   score.Result{name, value, label, ok}
                        │
                        ├─► events.Broker  → live dashboard
                        └─► telemetry      → control plane (numbers only)
```

### Data plane — structural scorers (`go/internal/score`)

Deterministic, dependency-free checks over a completed response. They answer
the questions that do not need a model:

| Scorer | Catches |
|---|---|
| `Empty` | the model returned nothing |
| `Truncated` | hit the token ceiling mid-answer (`finish_reason: length`) |
| `JSONValid` | structured-output workloads silently returning prose |
| `Refusal` | the model declined the task |
| `LatencyBudget` | responses slower than the session's budget |
| `CostBudget` | responses more expensive than the session's budget |

Each returns a `Result` carrying a numeric value and a pass/fail label. These
are cheap, run on every request, and catch a surprising share of real
production failure — an agent whose tool-call arguments stopped parsing as JSON
is broken long before anyone notices the output is wrong.

### Control plane — judged scorers (next)

Rubric scoring via LLM-as-judge, run against sessions the operator has
explicitly retained. Kept out of the data plane because it needs a model call,
a rubric store, and content — none of which belong in a latency-critical
stdlib-only proxy.

Judges are validated before they are trusted: a judge is itself scored against
human labels on a sample, and its agreement rate is reported alongside its
verdicts. An unvalidated judge is a number that feels like a measurement.

## Roadmap

**Phase 1 — score the stream.** Structural scorers in the data plane, results
on the live event stream and in telemetry. *(Package and tests landed; adapter
content extraction is the remaining wiring — see below.)*

**Phase 2 — harvest golden sets.** Promote real production sessions into a
regression dataset from the dashboard. This is the feature the proxy position
makes uniquely easy: the hardest part of evals is building a golden set that
reflects real traffic, and Turnstile already sees all of it. Retention is
opt-in and per-project, because of constraint 2.

**Phase 3 — regression gate.** `turnstile eval run` replays a golden set
against a new model or prompt version, diffs the scores against the recorded
baseline, and exits non-zero on regression. That makes it a CI gate rather than
a dashboard.

**Phase 4 — quality breaker.** Track score distribution per session and per
project; trip the existing breaker when quality degrades, not just when spend
does. The enforcement machinery already exists (`go/internal/enforce`); this
adds a signal to it.

## Remaining wiring for phase 1

The `Adapter` interface exposes `ParseUsage(sseData []byte) (Usage, bool)`,
which the proxy calls on candidate frames off the forwarding path. Scoring
needs the same treatment for content:

```go
// ParseContent extracts assistant text and the finish reason from a single
// SSE data payload or a complete non-streaming body. Returns false when the
// frame carries neither.
ParseContent(payload []byte) (Content, bool)
```

Implemented per adapter (OpenAI, Anthropic, Gemini, OpenRouter), accumulated
across frames exactly as usage already is, and handed to `score.Sink` when the
stream completes. The accumulation buffer is bounded by the existing
`MaxMeterBytes` ceiling so a large response cannot grow memory without limit,
and it is discarded once scored.

Until that lands, `score` is exercised directly by its tests and by any caller
that can supply an `Observation`.
