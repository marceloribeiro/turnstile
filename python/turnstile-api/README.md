# Turnstile — API (FastAPI)

The multi-tenant platform. Owns the PostgreSQL database and serves: **auth**, **organizations / members / invitations**, per-deployment **ingest keys**, the **telemetry ingest endpoint** the Go container posts to, and the **read API** the web dashboard consumes.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`docs/FLOWS.md`](docs/FLOWS.md) for the design and request flows.

## Stack

FastAPI · SQLAlchemy 2 · Alembic · PostgreSQL (psycopg3) · Pydantic v2 · PyJWT · bcrypt · Postmark (email) · **Redis** (pub/sub for live dashboard WebSocket) · pytest.

## Quickstart

```sh
just install          # venv (python3.13) + requirements
cp .env.example .env  # adjust DB_* etc.
just db-create-all    # turnstile_api_dev + turnstile_api_test
just db-migrate-all
just up               # uvicorn on :8000
```

`GET /health` (liveness), `GET /ready` (DB check). Swagger at `/docs` behind HTTP basic auth (`DOCS_USERNAME`/`DOCS_PASSWORD`).

## Key endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/register`, `/login` | — | returns user + JWT |
| GET | `/me` | JWT | current user |
| POST/GET | `/organizations` | JWT | create (→ owner) / list mine |
| GET | `/organizations/{id}/members` | JWT (member) | members |
| POST/GET | `/organizations/{id}/invitations` | JWT (admin) | invite (emails) / list pending |
| POST | `/invitations/accept` | JWT | accept by token |
| POST/GET | `/organizations/{id}/deployments` | JWT (admin) | mint ingest key (shown once) / list |
| GET | `/organizations/{id}/sessions`, `/summary` | JWT (member) | telemetry read (dollars-prevented) |
| POST | `/ingest/sessions` | ingest key | Go container posts session aggregates |
| WS | `/ws/organizations/{id}?token=` | JWT (member) | live telemetry pushes (Redis pub/sub) → dashboard updates without polling |

## Config (env)

`DB_USER/DB_PASS/DB_HOST/DB_PORT/DB_NAME` (or `DATABASE_URL`) · `JWT_SECRET/_ALGORITHM/_EXPIRE_MINUTES` · `DOCS_USERNAME/_PASSWORD` · `CORS_ORIGINS` (comma-separated; defaults to `http://localhost:3000` — **production MUST set the real dashboard origin(s)**, not `*`) · `APP_BASE_URL` (invite links) · `POSTMARK_API_TOKEN/_FROM/_MESSAGE_STREAM` · `REDIS_URL` (live-update pub/sub).

## Layout

```
app/
  config.py      pydantic-settings (env)
  database.py    engine, SessionLocal, TimestampedBase (uuid + timestamps + soft delete)
  deps.py        get_db, get_current_user (JWT), get_ingest_deployment (ingest key), docs_auth
  security.py    bcrypt + JWT
  mailer.py      Postmark HTTP send
  pubsub.py      Redis publish (live dashboard updates)
  main.py        app, CORS, routers, auth-gated /docs
  models/        users, organizations, organization_members/invitations, deployments, turnstile_sessions
  schemas/       pydantic request/response models
  services/      business logic — routes never touch the DB
  routers/       health, auth, organizations, invitations, telemetry, ingest, ws (WebSocket)
alembic/         migrations (YYYYMMDD_HHmmss_*.py)
tests/           pytest (per-test truncate isolation, test-DB guard)
```

## Develop

```sh
just test     # runs against turnstile_api_test (a guard refuses non-_test DBs)
just db-revision name="add something"   # alembic autogenerate
```
