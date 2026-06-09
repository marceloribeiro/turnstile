# Flows — Go Data Plane

## 1. Request lifecycle (the hot path)

```
client request (base URL → Turnstile :8080)
  │
  ▼ proxy.ServeHTTP
  read body ─────────────────────────────────────────── t0
  │
  adapter.Registry.Pick(req)         ── first Match() wins; OpenRouter fallback
  │
  session.Resolver.Resolve(headers, adapter.ExtractRequestMeta)
  │     header → anchor-hash inference → unattributed bucket
  │
  enforce.Pre(ident, body)  ◀── SYNCHRONOUS GATE
  │     kill? budget? loop?  →  blocked → adapter.NativeError (provider-shaped 4xx), STOP
  │
  ▼ allowed ──────────────────────────────────────────── t1 (pre-forward done)
  client.Do(upstream request)        ── forwards to adapter.ForwardURL
  │
  branch on response Content-Type:
  │  STREAMING (text/event-stream):
  │     write each line → flush  (forward FIRST, R1)      t2 = first byte, t3 = first forwarded
  │     if line has "usage" + "data:": adapter.ParseUsage (off the flush path)
  │     if enforcer.Killed(session) mid-stream → abort    (live gate)
  │  NON-STREAMING (application/json, e.g. plain chat/completions):
  │     read whole body → forward UNCHANGED (incl. gzip)
  │     for metering: gzip-decompress a COPY → adapter.ParseUsage(full JSON)
  ▼ done ─────────────────────────────────────────────── end
  if Debug: log one line (status, adapter, session, content-encoding, usage)
  async: metrics.Record(overhead = (t1-t0)+(t3-t2))      ── our slice, excludes upstream wait
  async: metering.Observe(ident, usage)  → pricing → meter store
```

Overhead is `(pre-forward work) + (first-byte forwarding)`, deliberately excluding the time blocked on the upstream. Measured ~6–11µs p50.

**Both response shapes are metered.** SSE streams yield usage in the terminal `data:` frame; non-streaming JSON carries `usage` in the single body. Providers gzip the JSON when the client sends `Accept-Encoding: gzip` (OpenRouter does), so for metering we decompress a *copy* — the client still receives the original bytes untouched. `TURNSTILE_DEBUG=true` logs whether usage was parsed per request (the `usage=true/false` and `ce=` fields), which is the fastest way to diagnose a `$0` dashboard.

## 2. Session resolution

1. `X-Turnstile-Session` header present → authoritative (`sess:<id>`).
2. Else infer: `HMAC(salt, hash(system+first-user message) + key-fingerprint + user)` → `infer:<hash>`. Stable across turns (same conversation root), de-collided by key + user.
3. Else (no anchorable content) → `unattributed:<key-fingerprint>` bucket with its own safety budget.

Native fields (`user`, `metadata.user_id`) are always extracted and linked → the session → user → app/key → org rollup ladder.

## 3. Enforcement

Three rules at the synchronous pre-gate:
- **Manual kill** — session marked killed (via control plane). Blocks.
- **Spend ceiling** — accumulated session cost ≥ budget. Blocks.
- **Loop detection** — count of identical request-body fingerprints in a rolling window ≥ threshold (default 20/60s). Blocks the runaway loop from the threshold-th call.

On a block: return a **provider-native error** (so the app's existing error handling catches it), record estimated **dollars prevented** (session mean cost/request), emit an event. Only that session is affected. A live gate also aborts an in-flight stream if the session is killed mid-response. All wrapped fail-open.

## 4. Metering → telemetry

```
usage parsed (async) → pricing.Cost(model, usage)        ── reported-cost wins
                     → meter.Store.Add(session aggregate)
                                   │
telemetry.Client (every TURNSTILE_INGEST_INTERVAL):
   store.Sessions() snapshot → POST {sessions:[...]} to TURNSTILE_INGEST_URL
                              header X-Turnstile-Ingest-Key
                              fail-open: errors logged + dropped, never blocks
```

The snapshot is field-aligned with the platform's ingest schema, so it posts verbatim. Only hashes + aggregates travel — never raw prompts or keys.

## 5. Control plane

`events.Broker` is a `metrics.Sink`, so every request/block becomes an event. The control plane serves `GET /v1/events` (SSE) plus REST (`/v1/sessions`, `/v1/summary`, `POST /v1/sessions/{id}/kill`). A kill via the API reaches `enforce.Kill`, and the data plane honors it on the next call (and aborts an in-flight stream).
