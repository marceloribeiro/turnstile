# Quick Start — Turnstile Go Data Plane

The proxy + circuit breaker. Sits between your app and the LLM provider. It runs standalone (enforcement works locally); connecting it to the platform is optional but powers the dashboard.

> **Want the whole stack** (your app → Turnstile → OpenRouter → metrics in the web app)? Follow the single end-to-end walkthrough: [`../docs/RUNNING_LOCALLY.md`](../docs/RUNNING_LOCALLY.md). It covers the correct startup order (the platform must be up and a key minted *before* Go).

## Prerequisites

- **Go 1.22+** (`brew install go`).
- **just** (`brew install just`).
- *(optional)* the **API running** on `:8000` if you want telemetry on the dashboard.

## Run it

```sh
cd go
cp .env.example .env           # tweak as needed (see below)
just install                   # builds (stdlib-only — no downloads)
just up                        # → data plane :8080, control plane :8081
just up 9000                   # → custom port: data :9000, control :9001
```

### Run one instance per project

Each project has its own ingest key, so run a separate interceptor per project —
a different **port** (recipe arg) and a different **key** (prepended env var, which
overrides `.env`). Space ports by 2 so the control planes (port+1) don't collide:

```sh
TURNSTILE_INGEST_KEY=ts_projectA just up 8080   # → data :8080, control :8081
TURNSTILE_INGEST_KEY=ts_projectB just up 8082   # → data :8082, control :8083
```

Then point each project's app at its port (`http://localhost:8080/api/v1`, `:8082/api/v1`, …).

Then **point your LLM client's base URL at `http://localhost:8080`** instead of the provider. Turnstile forwards each request to the right provider, meters it, and enforces budgets/loops.

```sh
# example: a streaming call through Turnstile to OpenRouter
curl -N http://localhost:8080/api/v1/chat/completions \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" -H "Content-Type: application/json" \
  -d '{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"stream":true,"max_tokens":8}'
```

> The provider API key travels in the client's `Authorization` header and is **passed through** — Turnstile never stores it. So the container itself needs no provider key.

## What to fill in `.env`

- `TURNSTILE_UPSTREAM` — fallback provider (default OpenRouter).
- `TURNSTILE_SESSION_BUDGET_USD`, `TURNSTILE_LOOP_*` — enforcement tuning.
- **To send telemetry to the dashboard:** set `TURNSTILE_INGEST_URL` (default `http://localhost:8000/ingest/sessions`) **and** `TURNSTILE_INGEST_KEY` — mint the key in the web app (organization → deployment → copy ingest key). Leave the key blank to run fully local (no telemetry).

## How it connects

- **Client app → Turnstile:** base URL `:8080`.
- **Turnstile → provider:** forwards per request (`TURNSTILE_UPSTREAM` / request-based routing).
- **Turnstile → platform:** posts hashed aggregates to the API's `/ingest/sessions` (ingest key auth). No DB credentials, no raw prompts.
- **Control plane:** `:8081` for live sessions + manual kill (`GET /v1/sessions`, `POST /v1/sessions/{id}/kill`).

## Common commands

```sh
just test          # all tests incl. the latency gate
just check         # vet + gofmt -l
```

See [`docs/FLOWS.md`](docs/FLOWS.md) for the request, enforcement, and telemetry flows.
