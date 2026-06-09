# Turnstile — Documentation

Turnstile is a local proxy that sits between your app and any OpenAI-compatible
endpoint (direct provider adapters landing), watches every AI session in real
time, and trips a circuit breaker the instant *one* session goes rogue — a
runaway agent loop or a blown budget — without taking down the rest of your app.

> **Status:** working end-to-end via OpenRouter today; direct OpenAI and
> Anthropic adapters in progress. See [ROADMAP.md](ROADMAP.md).

This directory holds cross-cutting documentation. Each service also ships its own
deeper docs.

## Start here

- **[RUNNING_LOCALLY.md](RUNNING_LOCALLY.md)** — bring up the whole stack
  end-to-end (your app → Turnstile → provider → metrics in the dashboard).
- **[ROADMAP.md](ROADMAP.md)** — what's built, what's next, and known follow-ups.

## The three services

| Service | Path | Deeper docs |
| --- | --- | --- |
| Go data plane (proxy + circuit breaker) | [`../go`](../go) | [`../go/docs/ARCHITECTURE.md`](../go/docs/ARCHITECTURE.md), [`../go/docs/FLOWS.md`](../go/docs/FLOWS.md) |
| FastAPI control plane (API + database) | [`../python/turnstile-api`](../python/turnstile-api) | [`../python/turnstile-api/docs/ARCHITECTURE.md`](../python/turnstile-api/docs/ARCHITECTURE.md), [`../python/turnstile-api/docs/FLOWS.md`](../python/turnstile-api/docs/FLOWS.md) |
| Next.js dashboard | [`../javascript/turnstile-web`](../javascript/turnstile-web) | [`../javascript/turnstile-web/docs/ARCHITECTURE.md`](../javascript/turnstile-web/docs/ARCHITECTURE.md), [`../javascript/turnstile-web/docs/FLOWS.md`](../javascript/turnstile-web/docs/FLOWS.md) |

## How it fits together

```
your app's LLM calls
   │  (base-URL swap — no SDK, no code changes)
   ▼
Go data plane ──── meters every session; kills rogue loops/budgets in real time (~microseconds overhead)
   │  posts hashed aggregates (never raw prompts/keys), authed by a per-deployment ingest key
   ▼
FastAPI control plane ──── upserts into PostgreSQL, scoped to the organization
   │
   ▼
Next.js dashboard ──── live sessions, spend, and dollars-prevented per organization
```

The data plane holds only an ingest key — never database credentials, never raw
prompts. See the root [README](../README.md) for the high-level overview.
