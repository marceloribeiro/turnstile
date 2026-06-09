# Architecture — Turnstile API

## Key tech

- **FastAPI** (monolith, one deployable) + **uvicorn**.
- **SQLAlchemy 2** (typed `Mapped[...]` models) + **Alembic** migrations + **PostgreSQL** via **psycopg3** (`postgresql+psycopg://`).
- **Pydantic v2** schemas (request/response validation, `from_attributes` for ORM → DTO).
- **PyJWT** (HS256 bearer tokens) + **bcrypt** (password hashing).
- **Postmark** HTTP API for transactional email (invitations).

## Layered structure (builder convention)

```
router  →  service  →  model/DB
  (thin)   (logic)     (SQLAlchemy)
```

- **routers/** — HTTP surface only; depend on `get_db`, `get_current_user`, `get_ingest_deployment`; call services; map to schemas.
- **services/** — all business logic and DB access (`user_service`, `org_service`, `invitation_service`, `deployment_service`, `telemetry_service`). The only layer that queries/commits.
- **models/** — SQLAlchemy models, all subclassing `TimestampedBase`.
- **schemas/** — Pydantic DTOs.
- **deps.py** — dependency-injection: DB session, JWT auth, ingest-key auth, docs basic auth.
- **security.py / mailer.py / config.py** — crypto, email, settings.

## Data model

```
users ──< organization_members >── organizations ──< deployments
  │                                      │                 │
  │                                      └──< organization_invitations
  │                                                        │
  └─ (auth)                              turnstile_sessions ┘  (org_id + deployment_id)
```

- **users** — first/last name, unique email, bcrypt `password_hash`.
- **organizations** — name, website_url.
- **organization_members** — user_id, organization_id, `role ∈ {member, admin, owner}` (unique per user+org).
- **organization_invitations** — email, role, opaque `token`, `status`, invited_by, accepted_at.
- **deployments** — org-scoped Go container identity; `ingest_key_hash` (sha256), `ingest_key_prefix` (display), `last_seen_at`.
- **turnstile_sessions** — durable telemetry written via ingest; aggregates only (session_key, source, model, token counts, cost, blocks, prevented), UPSERT by `(deployment_id, session_key)`.

## Authentication, two kinds

- **User JWT** (`get_current_user`) — bearer token for the web app and human API calls.
- **Ingest key** (`get_ingest_deployment`) — `X-Turnstile-Ingest-Key` header from the Go container; sha256-looked-up to a deployment → its org. Scopes all ingested telemetry to that org.

## Live updates (Redis pub/sub + WebSocket)

When telemetry is ingested, the API publishes to a per-org Redis channel (`pubsub.py`, sync
client, fail-safe). The WebSocket endpoint (`routers/ws.py`, `redis.asyncio`) authenticates a
JWT + org membership, subscribes to that channel, and relays each message to the dashboard so it
refreshes without polling. Redis is the bus, so this works across multiple uvicorn workers/hosts.
Run Redis with `just docker-up` (or a native redis); `REDIS_URL` configures it.

## Relationship to the rest of the system

This API is the **only DB writer**. The Go data plane posts aggregates to `/ingest/sessions`; the Next.js app reads `/organizations/{id}/sessions` + `/summary` and drives auth/org management. Privacy holds because only hashes/aggregates cross the ingest boundary — raw prompts and provider keys never reach this service.
