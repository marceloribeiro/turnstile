# Running the Full Stack Locally — End to End

Goal: your **existing app that talks to OpenRouter** → **Turnstile** → **OpenRouter**, with live metrics showing up in the **web dashboard**.

> ⚠️ **Order matters.** The Go container needs an **ingest key**, and that key is minted in the web app (organization → deployment). So bring up the **platform first** (API + Web), mint the key, **then** start Go. Starting Go before the key exists = telemetry silently disabled (you'd have to restart Go after).
>
> Correct order: **Postgres → API → Web → mint key → Go → point your app → use it.**

## 0. Prerequisites

- PostgreSQL on `localhost:5432` (Postgres.app or docker)
- Go 1.22+, Python 3.13, Node 18+, and `just` (`brew install go just`)

## 1. Start the API (`:8000`)

```sh
cd python/turnstile-api
cp .env.example .env          # fill DB_USER/DB_PASS to match your Postgres
just install
just db-create-all
just db-migrate-all
just up                       # → http://localhost:8000
```

## 2. Start the Web app (`:3000`)

```sh
cd javascript/turnstile-web
cp .env.example .env.local    # NEXT_PUBLIC_API_URL=http://localhost:8000 (default)
just install
just up                       # → http://localhost:3000
```

## 3. Mint an ingest key (in the web UI)

1. Open `http://localhost:3000` → **register** an account.
2. **Create an organization** → open it.
3. In the **Deployments** card → name it (e.g. `local`) → **Create & mint key**.
4. **Copy the `ts_…` key** shown in the green box (it's shown only once).

## 4. Start the Go data plane (`:8080`)

```sh
cd go
cp .env.example .env
# edit .env: paste the key from step 3
#   TURNSTILE_INGEST_KEY=ts_...
#   TURNSTILE_INGEST_URL=http://localhost:8000/ingest/sessions   (already the default)
#   (optional) TURNSTILE_INGEST_INTERVAL=5s   to see metrics sooner while testing
just install
just up                       # → data plane :8080, control plane :8081
```

On startup you should see: `telemetry: pushing to http://localhost:8000/ingest/sessions every …`.

## 5. Point your app at Turnstile

Your app currently uses OpenRouter's base URL `https://openrouter.ai/api/v1`. **Change only the host** — keep the `/api/v1` path and keep your OpenRouter API key (Turnstile passes it through and never stores it):

```
https://openrouter.ai/api/v1   →   http://localhost:8080/api/v1
```

Examples:

```python
# OpenAI SDK (python) pointed at OpenRouter-via-Turnstile
from openai import OpenAI
client = OpenAI(
    base_url="http://localhost:8080/api/v1",   # was https://openrouter.ai/api/v1
    api_key="<your OpenRouter key>",
)
```
```js
// OpenAI SDK (node)
const client = new OpenAI({
  baseURL: "http://localhost:8080/api/v1",
  apiKey: process.env.OPENROUTER_API_KEY,
});
```

> Keep the `/api/v1` — Turnstile forwards the path as-is to OpenRouter. Dropping it would forward to the wrong path.
>
> *(Optional)* For exact per-trajectory attribution, set an `X-Turnstile-Session: <your-id>` header per conversation. Without it, sessions still appear — grouped by content inference (source `inferred`).

## 6. Use your app → watch the metrics

1. Run your app so it makes a few LLM calls (now flowing through Turnstile → OpenRouter).
2. Wait one push interval (default **30s**, or whatever you set `TURNSTILE_INGEST_INTERVAL` to).
3. **Refresh your organization's page** in the web app → the **Sessions** table fills in, and the **Dollars prevented** hero + totals update.

You can also watch live, without the dashboard:
```sh
curl http://localhost:8081/v1/sessions   # control plane: current sessions
curl http://localhost:8081/v1/summary    # totals
```

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| **Calls succeed (200) but stats stay $0** | Run Go with **`TURNSTILE_DEBUG=true just up`** and re-trigger. Each request logs a line; check the `usage=` field. `usage=true` → metering works (it's a telemetry/org issue, see below). `usage=false` → the response wasn't parsed; note the `ce=` (content-encoding) + `model=` and report it. (Non-streaming JSON and gzip are handled.) |
| No sessions in the dashboard (but `:8081/v1/summary` shows them) | Telemetry/org issue. Did you set `TURNSTILE_INGEST_KEY` **before** starting Go? If not, restart Go. Wait one interval (or `TURNSTILE_INGEST_INTERVAL=5s`). View the org that key belongs to. |
| `:8081/v1/summary` says `unauthorized` | You set `TURNSTILE_CONTROL_TOKEN` — pass it: `-H "Authorization: Bearer <token>"` (or blank the token for localhost-open). |
| Go log shows `telemetry: disabled` | `TURNSTILE_INGEST_URL` and/or `TURNSTILE_INGEST_KEY` is empty in `.env`. |
| Go log: `ingest returned 401` | Ingest key doesn't match a deployment — re-mint and update `.env`. |
| Dashboard never updates on its own | Live updates need **Redis** running (`just docker-up` in the API) and the org page open; freshness is still bounded by `TURNSTILE_INGEST_INTERVAL`. |
| Your LLM calls fail / time out | Base URL must be `http://localhost:8080/api/v1` (keep `/api/v1`); your OpenRouter key valid. A client-side timeout (`ReadTimeout`) is *your app's* HTTP timeout vs slow generation — raise it; Turnstile adds ~µs and has no upstream timeout. |
| Metrics under the wrong org | Telemetry attributes to the org of the deployment whose key you used — view that org's page. |

**Golden rule for "$0":** check the **Go control plane first** (`:8081/v1/summary`) to see whether metering happened, *before* blaming the dashboard. `TURNSTILE_DEBUG=true` shows it per-request.

## What's connected to what

```
your app ──base URL :8080/api/v1──► Go data plane ──forwards──► openrouter.ai
                                       │ posts aggregates (ingest key)
                                       ▼
                                    API :8000 ──► Postgres :5432
                                       ▲ JWT
                                     Web :3000 ──► your dashboard
```
