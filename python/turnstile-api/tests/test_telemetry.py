from sqlalchemy import func, select

from app.models.turnstile_session import TurnstileSession
from app.models.user import User
from app.schemas.organization import OrganizationCreate
from app.schemas.telemetry import IngestSession
from app.security import hash_password
from app.services import deployment_service, org_service, telemetry_service


def register(client, email, password="pw123456"):
    r = client.post(
        "/register",
        json={"first_name": "T", "last_name": "U", "email": email, "password": password},
    )
    assert r.status_code == 201
    return r.json()["token"]


def auth(token):
    return {"Authorization": f"Bearer {token}"}


def setup_org(client, email):
    tok = register(client, email)
    org_id = client.post("/organizations", json={"name": "Co"}, headers=auth(tok)).json()["id"]
    return tok, org_id


def mint_key(client, tok, org_id, name="prod"):
    """Deployments are minted under a project now."""
    pid = client.post(
        f"/organizations/{org_id}/projects", json={"name": "Default"}, headers=auth(tok)
    ).json()["id"]
    return client.post(
        f"/organizations/{org_id}/projects/{pid}/deployments",
        json={"name": name},
        headers=auth(tok),
    ).json()["ingest_key"]


def test_create_deployment_returns_key_once(client):
    tok, org_id = setup_org(client, "dep@example.com")
    pid = client.post(
        f"/organizations/{org_id}/projects", json={"name": "Default"}, headers=auth(tok)
    ).json()["id"]
    r = client.post(
        f"/organizations/{org_id}/projects/{pid}/deployments",
        json={"name": "prod"},
        headers=auth(tok),
    )
    assert r.status_code == 201
    body = r.json()
    assert body["ingest_key"].startswith("ts_")
    assert body["ingest_key_prefix"].startswith("ts_")

    # listing deployments must NOT expose the plaintext key
    lst = client.get(f"/organizations/{org_id}/deployments", headers=auth(tok)).json()
    assert len(lst) == 1
    assert "ingest_key" not in lst[0]


def test_ingest_then_read_back(client):
    tok, org_id = setup_org(client, "ing@example.com")
    key = mint_key(client, tok, org_id)

    # Go-shaped snapshot (meter.Record fields, flattened).
    batch = {
        "sessions": [
            {
                "id": "sess:vip",
                "source": "header",
                "user": "u1",
                "key_fp": "abc",
                "model": "openai/gpt-4o-mini",
                "last_cost": "reported",
                "requests": 3,
                "prompt_tokens": 30,
                "completion_tokens": 24,
                "cost": 0.0021,
                "blocks": 1,
                "prevented": 0.0007,
            }
        ]
    }
    r = client.post("/ingest/sessions", json=batch, headers={"X-Turnstile-Ingest-Key": key})
    assert r.status_code == 200
    assert r.json()["upserted"] == 1

    sessions = client.get(f"/organizations/{org_id}/sessions", headers=auth(tok)).json()
    assert len(sessions) == 1
    s = sessions[0]
    assert s["session_key"] == "sess:vip" and s["cost"] == 0.0021 and s["blocks"] == 1

    summary = client.get(f"/organizations/{org_id}/summary", headers=auth(tok)).json()
    assert summary["sessions"] == 1
    assert summary["total_cost"] == 0.0021
    assert summary["dollars_prevented"] == 0.0007


def test_ingest_upserts_not_duplicates(client):
    tok, org_id = setup_org(client, "up@example.com")
    key = mint_key(client, tok, org_id, name="p")
    hdr = {"X-Turnstile-Ingest-Key": key}

    client.post("/ingest/sessions", json={"sessions": [{"id": "s1", "requests": 1, "cost": 0.1}]}, headers=hdr)
    client.post("/ingest/sessions", json={"sessions": [{"id": "s1", "requests": 5, "cost": 0.5}]}, headers=hdr)

    sessions = client.get(f"/organizations/{org_id}/sessions", headers=auth(tok)).json()
    assert len(sessions) == 1  # upsert, not duplicate
    assert sessions[0]["requests"] == 5 and sessions[0]["cost"] == 0.5  # absolute overwrite


def test_ingest_rejects_bad_key(client):
    r = client.post(
        "/ingest/sessions",
        json={"sessions": []},
        headers={"X-Turnstile-Ingest-Key": "ts_not-a-real-key"},
    )
    assert r.status_code == 401


def test_sessions_require_membership(client):
    _, org_id = setup_org(client, "owner@example.com")
    outsider = register(client, "outsider@example.com")
    assert client.get(f"/organizations/{org_id}/sessions", headers=auth(outsider)).status_code == 403


def test_org_cannot_read_other_orgs_telemetry(client):
    # Org A ingests telemetry; an Org B member must never see it, even by passing
    # Org A's project_id as a query filter.
    tok_a, org_a = setup_org(client, "tenant-a@example.com")
    key_a = mint_key(client, tok_a, org_a)
    client.post(
        "/ingest/sessions",
        json={"sessions": [{"id": "secret", "cost": 9.0}]},
        headers={"X-Turnstile-Ingest-Key": key_a},
    )

    tok_b, org_b = setup_org(client, "tenant-b@example.com")
    # B reading its own (empty) org: fine, but no data.
    own = client.get(f"/organizations/{org_b}/summary", headers=auth(tok_b)).json()
    assert own["total_cost"] == 0.0
    # B reading A's org id outright: forbidden (not a member).
    assert client.get(f"/organizations/{org_a}/summary", headers=auth(tok_b)).status_code == 403


def test_member_cannot_mint_deployment(client):
    # Provisioning ingest keys is admin-only.
    tok, org_id = setup_org(client, "kown@example.com")
    pid = client.post(
        f"/organizations/{org_id}/projects", json={"name": "P"}, headers=auth(tok)
    ).json()["id"]
    client.post(
        f"/organizations/{org_id}/invitations",
        json={"email": "plain@example.com", "role": "member"},
        headers=auth(tok),
    )
    member = register(client, "plain@example.com")
    r = client.post(
        f"/organizations/{org_id}/projects/{pid}/deployments",
        json={"name": "x"},
        headers=auth(member),
    )
    assert r.status_code == 403


def test_member_cannot_list_invitations(client):
    # Pending invitations (emails) are admin-only to read.
    tok, org_id = setup_org(client, "iown@example.com")
    client.post(
        f"/organizations/{org_id}/invitations",
        json={"email": "plain@example.com", "role": "member"},
        headers=auth(tok),
    )
    member = register(client, "plain@example.com")
    r = client.get(f"/organizations/{org_id}/invitations", headers=auth(member))
    assert r.status_code == 403


def test_ingest_requires_key_header(client):
    # Missing the ingest-key header entirely is a 422 (required header), not a 500.
    r = client.post("/ingest/sessions", json={"sessions": []})
    assert r.status_code == 422


def test_upsert_sessions_inserts_then_updates_atomically(db):
    # Direct service test of the ON CONFLICT UPSERT path: a new key inserts, the
    # same key re-ingested updates in place (no duplicate, no IntegrityError).
    user = User(
        first_name="T", last_name="U", email="upsert@example.com",
        password_hash=hash_password("pw123456"),
    )
    db.add(user)
    db.commit()
    db.refresh(user)
    org = org_service.create_organization(db, user, OrganizationCreate(name="Up"))
    deployment, _ = deployment_service.create_deployment(db, org.id, "d")

    def _count():
        return db.execute(
            select(func.count(TurnstileSession.id)).where(
                TurnstileSession.deployment_id == deployment.id
            )
        ).scalar_one()

    n = telemetry_service.upsert_sessions(
        db, deployment, [IngestSession(id="sk1", requests=1, cost=0.1)]
    )
    assert n == 1 and _count() == 1

    # Same key again: updates in place (absolute overwrite), no duplicate row.
    telemetry_service.upsert_sessions(
        db, deployment, [IngestSession(id="sk1", requests=9, cost=0.9)]
    )
    assert _count() == 1
    row = db.execute(
        select(TurnstileSession).where(TurnstileSession.session_key == "sk1")
    ).scalar_one()
    assert row.requests == 9 and row.cost == 0.9

    # A brand-new key inserts alongside.
    telemetry_service.upsert_sessions(
        db, deployment, [IngestSession(id="sk2", requests=2)]
    )
    assert _count() == 2
