# Roadmap

A snapshot of what's built and what's planned. This is engineering-focused; it is
not a product commitment.

## Built

- **Go data plane** — transparent streaming proxy with a sacred latency gate,
  provider adapters for **OpenRouter, OpenAI** (Chat Completions + Responses API,
  with `previous_response_id` chain resolution), **Anthropic** (Messages API), and
  **Gemini** (Generative Language API), session resolution (header → anchor-hash
  inference → per-key bucket), metering + pricing, enforcement (per-session budget
  ceiling, loop detection, manual kill), control-plane REST + SSE API, and a
  telemetry client that ships hashed aggregates. Stdlib-only, fail-open,
  self-instrumented.
- **FastAPI control plane** — multi-tenant auth (JWT), organizations / members /
  invitations, projects, per-deployment ingest keys (sha256-hashed), the
  telemetry ingest endpoint, the dashboard read API, and live updates over Redis
  pub/sub + WebSocket. PostgreSQL via SQLAlchemy 2 + Alembic.
- **Next.js dashboard** — auth, organization and project management, ingest-key
  minting, and a live telemetry dashboard with "dollars prevented" as the hero
  metric.

## Planned

### Evals — scoring production sessions
Turnstile already sees every request and response, which makes it the one place
that can evaluate production traffic with no application instrumentation. The
end state is a breaker that trips on a **quality** regression, not just on cost
or loop detection. Structural scorers and the design have landed
(`go/internal/score`, [docs/EVALS.md](EVALS.md)); adapter content extraction,
golden-set harvesting, a CI regression gate and the quality breaker follow.

### Provider adapters
Direct adapters for **OpenAI, Anthropic, and Gemini** now ship alongside the
OpenRouter fallback. The adapter interface (`go/internal/adapter`) is the
extension point; one running instance serves all providers and routes by request
shape. Further providers (e.g. Bedrock, Vertex) slot in the same way.

### Packaging & deployment
- Docker image for the data plane and published install/runbook docs.
- **One-command local stack.** A `docker compose up` that boots Postgres, Redis,
  the API, the web app, and the data plane, and auto-mints a dev ingest key and
  wires it into the data plane — collapsing the current multi-step setup
  (API → web → mint key → Go) into a single command for trials.

### Dashboard
- A manual **kill-switch** control in the UI. The control plane and API already
  support killing a session (`POST /v1/sessions/{id}/kill`); the dashboard does
  not yet expose a button for it.

## Engineering follow-ups (from the code audit)

These were deliberately deferred as architectural changes rather than quick fixes:

- **Web — auth token storage.** The JWT is stored in `localStorage`, which is
  readable by any XSS. Moving to an httpOnly cookie is the standard hardening but
  changes the auth flow on both ends.
- **Web — data fetching.** Reads currently run in client-side effects. Moving them
  to Server Components / route handlers would align with Next 16 and the React
  Compiler rules, and remove the awaited-effect workarounds.
- **Web — request cancellation.** Thread an `AbortController` through `lib/api.ts`
  so in-flight requests cancel on route change / unmount.
- **API — JWT claims.** Add `nbf` / issuer / audience and a revocation or
  short-expiry-plus-refresh strategy if server-side logout is needed.
