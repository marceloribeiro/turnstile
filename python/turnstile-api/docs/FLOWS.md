# Flows — Turnstile API

## 1. Auth

```
POST /register {first_name,last_name,email,password}
   user_service.create_user → bcrypt hash, insert, AUTO-ACCEPT pending invitations (rule 1)
   → returns {user, token}   (JWT, sub=user.id)

POST /login {email,password}
   user_service.authenticate → verify bcrypt → {user, token}

GET /me  (Authorization: Bearer <jwt>)
   deps.get_current_user decodes JWT → loads user → returns it
```

## 2. Organizations & invitations (ORGANIZATION_BASED rules)

```
POST /organizations {name}
   org_service.create_organization → insert org + add_member(creator, role=owner)   ← RULE 2

POST /organizations/{id}/invitations {email, role}        (caller must be admin/owner)
   invitation_service.create_invitation → token + insert + mailer.send_email(accept link)

Accept — two paths:
 (a) invitee already had an account:
     POST /invitations/accept {token}  → verify token's email == caller's email
        → add_member(role) + mark accepted
 (b) invitee signs up AFTER being invited:
     POST /register → user_service.create_user → auto_accept_on_signup            ← RULE 1
        → every pending invitation for that email becomes a membership
```

Role enforcement lives in services (`require_member`, `require_admin` by `ROLE_RANK`).

## 3. Telemetry ingest (Go → platform)

```
Admin: POST /organizations/{id}/deployments {name}
   deployment_service.create_deployment → generate "ts_..." key,
   store sha256(key) + prefix, return PLAINTEXT KEY ONCE (DeploymentCreated.ingest_key)

Go container (configured with that key):
   POST /ingest/sessions   header X-Turnstile-Ingest-Key: ts_...
        body {sessions:[ <meter.Record-shaped> ]}
   deps.get_ingest_deployment → sha256 lookup → deployment (→ org)
   telemetry_service.upsert_sessions → UPSERT by (deployment_id, session_key),
        absolute overwrite (snapshot), bump deployment.last_seen_at
```

> Note: ingest is **absolute overwrite** of aggregates (a snapshot). A Go restart resets its in-memory counters, so pre-restart history can be lost — delta ingest is a future refinement.

## 4. Dashboard read (platform → web)

```
GET /organizations/{id}/summary   (member)
   telemetry_service.summary → SUM(cost), SUM(prevented = dollars-prevented hero),
                               SUM(blocks), SUM(requests), COUNT(sessions)

GET /organizations/{id}/sessions  (member)
   telemetry_service.list_sessions → rows ordered by cost desc
```

## 5. Live dashboard updates (Redis pub/sub + WebSocket)

```
POST /ingest/sessions (Go)
   telemetry_service.upsert_sessions → pubsub.publish_telemetry(org_id)
        └─ Redis PUBLISH org:<id>:telemetry  {"type":"telemetry","upserted":n}

GET /ws/organizations/{id}?token=<jwt>   (browser dashboard)
   _authorize (JWT + membership) → SUBSCRIBE org:<id>:telemetry (redis.asyncio)
        └─ on message → relay to the WebSocket → the browser refetches /summary + /sessions
```

Redis is the bus, so this works across multiple uvicorn workers/hosts. Publishing is fail-safe
(a Redis outage never breaks ingest). Run Redis with `just docker-up` (or a native redis).

## 6. End-to-end (the whole system)

real LLM call → Go meters it → Go posts aggregates to `/ingest/sessions` → UPSERT into Postgres
→ **publish to Redis → WebSocket pushes to the dashboard (live)** → it refetches `/summary` + `/sessions`.
Demonstrated live, cross-process.
