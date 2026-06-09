def test_health_is_unauthenticated_and_ok(client):
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_ready_checks_db(client):
    r = client.get("/ready")
    assert r.status_code == 200
    assert r.json() == {"status": "ready"}


def test_docs_requires_basic_auth(client):
    assert client.get("/docs").status_code == 401
    assert client.get("/openapi.json").status_code == 401


def test_cors_reflects_allowed_origin_without_credentials(client):
    from app.main import origins

    # Use whatever the app is actually configured to allow (env-driven); a
    # wildcard reflects "*", an explicit origin reflects that origin.
    origin = "http://localhost:3000" if origins == ["*"] else origins[0]
    r = client.get("/health", headers={"Origin": origin})
    assert r.status_code == 200
    expected = "*" if origins == ["*"] else origin
    assert r.headers["access-control-allow-origin"] == expected
    # Bearer-token auth: credentials must NOT be enabled (the header is absent
    # when allow_credentials is False).
    assert "access-control-allow-credentials" not in r.headers


def test_cors_default_origin_is_not_wildcard():
    # The code default (independent of any on-disk .env override) must be the
    # local dashboard origin, not a wildcard.
    from app.config import Settings

    assert Settings.model_fields["CORS_ORIGINS"].default == "http://localhost:3000"
