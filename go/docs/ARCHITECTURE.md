# Architecture — Go Data Plane

## Key tech

- **Go (stdlib only).** Single static binary, ideal for a high-throughput streaming proxy. No external dependencies.
- **`net/http`** for both the proxy and the control plane. The proxy streams SSE by hand (`bufio.Reader.ReadBytes('\n')` + `http.Flusher`) rather than buffering, so a token is forwarded the instant it arrives.
- **`crypto/hmac` + `sha256`** for salted, non-reversible session keys and API-key fingerprints.
- **In-memory state** (sharded-free maps + mutexes); durable history is delegated to the platform via telemetry, not stored here.

## Two planes, one process

The binary exposes two HTTP listeners with different security postures:

- **Data plane** (`:8080`) — the proxy. Latency-critical, in the hot path of every LLM call. Takes app traffic.
- **Control plane** (`127.0.0.1:8081`) — dashboard data + commands (sessions, spend, kill). Auth-gated, localhost-bound by default. Low traffic, human-facing.

They share state (the meter store + enforcer + event broker) but never block each other.

## Components (`internal/`)

| Package | Responsibility |
|---------|----------------|
| `config` | Env-driven configuration with defaults. |
| `proxy` | The transparent streaming proxy. Reads body, routes via the adapter registry, resolves session, runs the **synchronous enforcement gate**, forwards, streams the response back unbuffered, and (async) meters + records. Owns the latency budget. |
| `adapter` | `Adapter` interface (`Name`/`Match`/`ForwardURL`/`ExtractRequestMeta`/`ParseUsage`/`NativeError`) + `Registry` (first-match, fallback). `openrouter/` is the first implementation. |
| `session` | Resolves a request to a trajectory: `X-Turnstile-Session` header → anchor-hash inference (`hash(system+first-user) + key-fp + user`) → per-key unattributed bucket. Always links native fields for rollups. |
| `pricing` | Cost engine. Precedence: provider-reported cost → customer override → loaded table → unknown. Loads OpenRouter's `/models` table. |
| `meter` | Accumulates per-session spend (`Record` aggregates) behind a `Store` interface (`MemStore` in-memory). Also satisfies the enforcer's `Ledger`. |
| `enforce` | The circuit breaker. Synchronous `Pre` gate (manual kill, spend ceiling, loop detection) + live mid-stream kill. Records dollars-prevented. |
| `control` | Control-plane REST + SSE: list/get sessions, summary, kill/unkill, live event stream. Auth + CORS middleware. |
| `events` | In-process pub/sub `Broker` that is *also* a `metrics.Sink`, so every request/block becomes a live event. |
| `metrics` | Self-instrumentation: measures *our* in-process overhead (excludes upstream wait) — the Phase 0 fix. `MultiSink` fans samples to recorder + broker. |
| `telemetry` | Background HTTP client that snapshots the meter store and POSTs aggregates to the platform's ingest endpoint, authed by an ingest key. Fail-open. |

## The adapter pattern (extension point)

A single Turnstile instance serves all providers. For each request the `Registry` calls every adapter's `Match()` and uses the first that claims it (OpenRouter is the fallback). The adapter then provides the upstream URL and the provider-specific parsing. This is how new providers (e.g. direct OpenAI and Anthropic) are added without touching the proxy — each new provider is one `Adapter` implementation.

## The latency principle (why it's fast)

Only the **pre-execution enforcement check** is synchronous. Session resolution feeds it, so it's synchronous too. Everything after the response — usage parsing, cost calc, metering, event emission, telemetry — runs in async goroutines, recover-guarded. The `metrics` package measures the synchronous slice in isolation; CI asserts it stays within p50 < 5ms / p99 < 15ms against an instant stub.
