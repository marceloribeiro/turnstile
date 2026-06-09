def register(client, email, password="pw123456"):
    r = client.post(
        "/register",
        json={"first_name": "P", "last_name": "U", "email": email, "password": password},
    )
    assert r.status_code == 201
    return r.json()["token"]


def auth(token):
    return {"Authorization": f"Bearer {token}"}


def setup(client, email):
    tok = register(client, email)
    org_id = client.post("/organizations", json={"name": "Co"}, headers=auth(tok)).json()["id"]
    return tok, org_id


def test_create_and_list_projects(client):
    tok, org_id = setup(client, "proj@example.com")
    r = client.post(
        f"/organizations/{org_id}/projects",
        json={"name": "Checkout", "description": "checkout agents"},
        headers=auth(tok),
    )
    assert r.status_code == 201
    assert r.json()["name"] == "Checkout"

    lst = client.get(f"/organizations/{org_id}/projects", headers=auth(tok)).json()
    assert [p["name"] for p in lst] == ["Checkout"]


def test_key_minted_under_project_attributes_telemetry(client):
    tok, org_id = setup(client, "scope@example.com")
    pid = client.post(
        f"/organizations/{org_id}/projects", json={"name": "Alpha"}, headers=auth(tok)
    ).json()["id"]

    key = client.post(
        f"/organizations/{org_id}/projects/{pid}/deployments",
        json={"name": "prod"},
        headers=auth(tok),
    ).json()["ingest_key"]

    # ingest telemetry with that key
    client.post(
        "/ingest/sessions",
        json={"sessions": [{"id": "sess:a", "requests": 5, "cost": 0.5, "prevented": 1.0}]},
        headers={"X-Turnstile-Ingest-Key": key},
    )

    # the session is attributed to the project
    sessions = client.get(f"/organizations/{org_id}/projects/{pid}/sessions", headers=auth(tok)).json()
    assert len(sessions) == 1
    assert sessions[0]["project_id"] == pid

    # project summary is scoped
    psum = client.get(f"/organizations/{org_id}/projects/{pid}/summary", headers=auth(tok)).json()
    assert psum["sessions"] == 1 and psum["total_cost"] == 0.5

    # org summary filtered by project_id matches
    fsum = client.get(
        f"/organizations/{org_id}/summary?project_id={pid}", headers=auth(tok)
    ).json()
    assert fsum["total_cost"] == 0.5


def test_project_isolation(client):
    tok, org_id = setup(client, "iso@example.com")
    p1 = client.post(f"/organizations/{org_id}/projects", json={"name": "One"}, headers=auth(tok)).json()["id"]
    p2 = client.post(f"/organizations/{org_id}/projects", json={"name": "Two"}, headers=auth(tok)).json()["id"]
    k1 = client.post(f"/organizations/{org_id}/projects/{p1}/deployments", json={"name": "d"}, headers=auth(tok)).json()["ingest_key"]
    k2 = client.post(f"/organizations/{org_id}/projects/{p2}/deployments", json={"name": "d"}, headers=auth(tok)).json()["ingest_key"]

    client.post("/ingest/sessions", json={"sessions": [{"id": "s1", "cost": 1.0}]}, headers={"X-Turnstile-Ingest-Key": k1})
    client.post("/ingest/sessions", json={"sessions": [{"id": "s2", "cost": 2.0}]}, headers={"X-Turnstile-Ingest-Key": k2})

    s1 = client.get(f"/organizations/{org_id}/projects/{p1}/summary", headers=auth(tok)).json()
    s2 = client.get(f"/organizations/{org_id}/projects/{p2}/summary", headers=auth(tok)).json()
    org = client.get(f"/organizations/{org_id}/summary", headers=auth(tok)).json()
    assert s1["total_cost"] == 1.0
    assert s2["total_cost"] == 2.0
    assert org["total_cost"] == 3.0  # org-wide sees both


def test_member_cannot_create_project(client):
    tok, org_id = setup(client, "owner@example.com")
    # add a plain member
    client.post(f"/organizations/{org_id}/invitations", json={"email": "m@example.com", "role": "member"}, headers=auth(tok))
    member = register(client, "m@example.com")
    r = client.post(f"/organizations/{org_id}/projects", json={"name": "X"}, headers=auth(member))
    assert r.status_code == 403
