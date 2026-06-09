import json
import uuid

import pytest
import redis as redislib
from starlette.websockets import WebSocketDisconnect

from app import pubsub
from app.config import settings


def _redis_up() -> bool:
    try:
        redislib.Redis.from_url(settings.REDIS_URL).ping()
        return True
    except Exception:
        return False


pytestmark = pytest.mark.skipif(not _redis_up(), reason="redis not running")


def _register(client, email):
    return client.post(
        "/register",
        json={"first_name": "W", "last_name": "S", "email": email, "password": "pw123456"},
    ).json()["token"]


def _setup_org(client):
    token = _register(client, f"ws{uuid.uuid4().hex[:8]}@ex.com")
    org_id = client.post(
        "/organizations", json={"name": "WSCo"}, headers={"Authorization": f"Bearer {token}"}
    ).json()["id"]
    return token, org_id


def test_ws_rejects_bad_token(client):
    with pytest.raises(WebSocketDisconnect):
        with client.websocket_connect(f"/ws/organizations/{uuid.uuid4()}?token=not-a-jwt"):
            pass


def test_ws_rejects_non_member(client):
    _, org_id = _setup_org(client)
    outsider = _register(client, f"out{uuid.uuid4().hex[:8]}@ex.com")
    with pytest.raises(WebSocketDisconnect):
        with client.websocket_connect(f"/ws/organizations/{org_id}?token={outsider}"):
            pass


def test_ws_receives_telemetry_event(client):
    token, org_id = _setup_org(client)
    with client.websocket_connect(f"/ws/organizations/{org_id}?token={token}") as wsc:
        # subscription is active once connected → publish reaches us
        pubsub.publish_telemetry(org_id, upserted=3)
        msg = json.loads(wsc.receive_text())
    assert msg["type"] == "telemetry"
    assert msg["org_id"] == org_id
    assert msg["upserted"] == 3
