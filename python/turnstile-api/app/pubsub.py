"""Redis pub/sub for live dashboard updates.

When telemetry is ingested, the API publishes to a per-organization channel; the
WebSocket endpoint (one subscriber per connected dashboard) relays it to the
browser. Redis is the bus, so this works across multiple uvicorn workers / hosts.

Publishing happens from synchronous request handlers (a sync redis client);
subscribing happens in the async WebSocket endpoint (redis.asyncio)."""

import json
import logging
import uuid

import redis

from .config import settings

log = logging.getLogger("turnstile.pubsub")

_client: redis.Redis | None = None


def _sync_client() -> redis.Redis:
    global _client
    if _client is None:
        _client = redis.Redis.from_url(settings.REDIS_URL)
    return _client


def org_channel(org_id: uuid.UUID | str) -> str:
    return f"org:{org_id}:telemetry"


def publish_telemetry(org_id: uuid.UUID | str, upserted: int = 0) -> None:
    """Notify dashboards watching this org that telemetry changed. Fail-safe — a
    Redis outage must never break ingest."""
    msg = json.dumps({"type": "telemetry", "org_id": str(org_id), "upserted": upserted})
    try:
        _sync_client().publish(org_channel(org_id), msg)
    except Exception as exc:  # noqa: BLE001
        log.warning("pubsub publish failed (ignored): %s", exc)
