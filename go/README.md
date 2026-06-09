# Turnstile — Go Data Plane

The hot-path engine. A transparent streaming proxy that sits between your app and any LLM provider, meters every AI session in real time, and trips a circuit breaker the instant *one* session goes rogue — a runaway agent loop or a blown budget — **without taking down the rest of your app**.

- **Drop-in:** swap one base URL. No SDK, no code changes.
- **Session-precise:** kills the one rogue trajectory, never the global API key.
- **Real-time:** pre-execution enforcement, not retrospective dashboards.
- **Negligible overhead:** ~6µs median added latency (measured, self-instrumented).
- **Fail-open:** a Turnstile fault never breaks the customer's app.
- **Private:** raw prompts and API keys are never persisted — only salted hashes and aggregates.

## Quickstart

```sh
# point your LLM client's base URL at :8080; Turnstile forwards to the provider
TURNSTILE_UPSTREAM=https://openrouter.ai go run ./cmd/turnstile

# with enforcement + telemetry to the platform:
TURNSTILE_SESSION_BUDGET_USD=5 \
TURNSTILE_INGEST_URL=https://api.example.com/ingest/sessions \
TURNSTILE_INGEST_KEY=ts_... \
go run ./cmd/turnstile
```

Two listeners come up: the **data plane** (`:8080`, the proxy) and the **control plane** (`127.0.0.1:8081`, dashboard data + manual kill).

## Configuration (env vars)

| Var | Default | Purpose |
|-----|---------|---------|
| `TURNSTILE_LISTEN` | `:8080` | Data-plane (proxy) listen address |
| `TURNSTILE_UPSTREAM` | `https://openrouter.ai` | Fallback provider upstream (OpenRouter adapter; handles `/api/v1/` and unmatched traffic) |
| `TURNSTILE_OPENAI_BASE` | `https://api.openai.com` | Direct OpenAI adapter upstream (claims the `/v1/` surface: `/v1/chat/completions` + `/v1/responses`) |
| `TURNSTILE_ANTHROPIC_BASE` | `https://api.anthropic.com` | Direct Anthropic adapter upstream (the Messages API, `/v1/messages`; matched ahead of OpenAI) |
| `TURNSTILE_GEMINI_BASE` | `https://generativelanguage.googleapis.com` | Direct Gemini adapter upstream (the Generative Language API, `/v1beta/models/{model}:generateContent`) |
| `TURNSTILE_FAIL_CLOSED` | `false` | Invert fail-open (block on internal error) |
| `TURNSTILE_SALT` | random | HMAC salt for key fingerprints + session hashes |
| `TURNSTILE_SESSION_BUDGET_USD` | `0` (off) | Per-session spend ceiling |
| `TURNSTILE_LOOP_ENABLED` / `_THRESHOLD` / `_WINDOW` | `true` / `20` / `60s` | Loop detection |
| `TURNSTILE_CONTROL_LISTEN` / `_TOKEN` | `127.0.0.1:8081` / "" | Control plane addr + optional bearer auth |
| `TURNSTILE_DASHBOARD_ORIGIN` | `http://localhost:3000` | CORS allowed origin for the dashboard; set your real origin in production (`*` allows any, not recommended) |
| `TURNSTILE_INGEST_URL` / `_KEY` / `_INTERVAL` | "" / "" / `30s` | Telemetry to the platform (disabled unless URL+key set) |
| `TURNSTILE_MAX_METER_BYTES` | `8388608` (8 MiB) | Cap on the non-streaming body buffered for metering; the client always gets the full body |
| `TURNSTILE_DEBUG` | `false` | Log one line per proxied request (status, adapter, session, content-encoding, whether usage parsed) |

## Layout

```
cmd/turnstile/         entrypoint — wires everything, two listeners, graceful shutdown
internal/
  config/              env-driven config
  proxy/               the transparent streaming proxy (the hot path)
  adapter/             ProviderAdapter interface + Registry
    openrouter/        fallback adapter (/api/v1/, in-stream cost)
    openai/            direct OpenAI adapter (/v1/ — Chat Completions + Responses API)
    anthropic/         direct Anthropic adapter (/v1/messages — Messages API)
    gemini/            direct Gemini adapter (/v1beta/models/{model}:generateContent)
  session/             session resolution (header → anchor-hash → unattributed)
  pricing/             cost engine (provider-reported → override → table)
  meter/               per-session spend accumulation (in-memory Store)
  enforce/             the circuit breaker (kill / budget / loop)
  control/             control-plane HTTP API (REST + SSE)
  events/              in-process pub/sub feeding the SSE stream
  metrics/             self-instrumentation (our overhead, not provider latency)
  telemetry/           HTTP client shipping aggregates to the platform
```

## Develop

```sh
go test ./...      # all packages; the proxy package asserts the latency gate
go vet ./...
gofmt -l .
```

Deeper docs: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) (tech + components) and [`docs/FLOWS.md`](docs/FLOWS.md) (request, enforcement, telemetry flows). To run the whole stack end-to-end, see the repo-level [`docs/RUNNING_LOCALLY.md`](../docs/RUNNING_LOCALLY.md).
