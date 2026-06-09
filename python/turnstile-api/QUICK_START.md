# Quick Start — Turnstile API

The FastAPI backend. Owns Postgres; serves auth, orgs, invitations, telemetry ingest, and the dashboard read API. **Start this first** — the web app and the Go container both connect to it.

> **Running the whole stack end-to-end** (your app → Turnstile → OpenRouter → metrics in the web app)? See [`../../docs/RUNNING_LOCALLY.md`](../../docs/RUNNING_LOCALLY.md).

## Prerequisites

- **PostgreSQL** running on `localhost:5432` (Postgres.app or `docker`).
- **Python 3.13** (`python3.13` on PATH).
- **just** (`brew install just`).
- **Redis** (for live dashboard updates): `just docker-up`, or a native redis on `:6379`.

## Run it

```sh
cd python/turnstile-api
cp .env.example .env          # then fill DB_USER/DB_PASS (and Postmark if you want emails)
just install                  # venv + dependencies
just db-create-all            # creates turnstile_api_dev + turnstile_api_test
just db-migrate-all           # applies migrations
just up                       # → http://localhost:8000
```

Verify:
```sh
curl localhost:8000/health    # {"status":"ok"}
curl localhost:8000/ready     # {"status":"ready"}  (checks the DB)
```
Swagger UI: `http://localhost:8000/docs` (basic auth — `DOCS_USERNAME`/`DOCS_PASSWORD`, default `turnstile`/`turnstile`).

## What to fill in `.env`

- `DB_USER`, `DB_PASS`, `DB_NAME` — your Postgres role/db. (Defaults assume a local superuser named `marcelo`, no password.)
- `JWT_SECRET` — change for anything non-local.
- `POSTMARK_API_TOKEN` + `POSTMARK_FROM` — only if you want invitation **emails** to actually send (blank = invitations still created, just not emailed).
- `APP_BASE_URL` — the web app URL used in invite links (default `http://localhost:3000`).

## How it connects

- **Web app** reaches it via `NEXT_PUBLIC_API_URL` (default `http://localhost:8000`).
- **Go container** posts telemetry to `POST /ingest/sessions` using a per-deployment **ingest key** (minted via the web app or `POST /organizations/{id}/deployments`).

## Common commands

```sh
just test          # pytest against turnstile_api_test
just db            # psql shell on the dev DB
just db-revision name="add x"   # new alembic migration
```

See [`docs/FLOWS.md`](docs/FLOWS.md) for the auth / org / ingest flows.
