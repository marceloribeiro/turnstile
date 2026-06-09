def _register(client, email="ada@example.com", password="secret123"):
    return client.post(
        "/register",
        json={"first_name": "Ada", "last_name": "Lovelace", "email": email, "password": password},
    )


def test_register_returns_user_and_token(client):
    r = _register(client)
    assert r.status_code == 201
    body = r.json()
    assert body["user"]["email"] == "ada@example.com"
    assert body["token"]
    assert "password" not in body["user"]
    assert "password_hash" not in body["user"]


def test_register_duplicate_email_conflicts(client):
    assert _register(client, email="dup@example.com").status_code == 201
    assert _register(client, email="dup@example.com").status_code == 409


def test_login_then_me(client):
    _register(client, email="grace@example.com", password="hopper99")
    r = client.post("/login", json={"email": "grace@example.com", "password": "hopper99"})
    assert r.status_code == 200
    token = r.json()["token"]

    me = client.get("/me", headers={"Authorization": f"Bearer {token}"})
    assert me.status_code == 200
    assert me.json()["email"] == "grace@example.com"


def test_login_wrong_password(client):
    _register(client, email="x@example.com", password="rightpass")
    r = client.post("/login", json={"email": "x@example.com", "password": "wrongpass"})
    assert r.status_code == 401


def test_me_requires_auth(client):
    assert client.get("/me").status_code in (401, 403)


def test_me_rejects_garbage_token(client):
    r = client.get("/me", headers={"Authorization": "Bearer not-a-jwt"})
    assert r.status_code == 401
