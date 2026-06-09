"""WebSocket endpoint for live dashboard updates.

A connected dashboard subscribes (via Redis pub/sub) to its organization's
telemetry channel and receives a message whenever new telemetry is ingested, so
the UI updates without polling. Auth is by JWT in a query param (browsers can't
set headers on a WebSocket handshake)."""

import asyncio
import uuid

import jwt
import redis.asyncio as aioredis
from fastapi import APIRouter, Query, WebSocket, WebSocketDisconnect, status

from ..config import settings
from ..database import SessionLocal
from ..pubsub import org_channel
from ..security import decode_access_token
from ..services import org_service, user_service

router = APIRouter()


def _authorize(org_id: uuid.UUID, token: str) -> bool:
    """JWT valid + user is a member of the org."""
    try:
        user_id = uuid.UUID(decode_access_token(token)["sub"])
    except (jwt.PyJWTError, KeyError, ValueError):
        return False
    db = SessionLocal()
    try:
        user = user_service.get_by_id(db, user_id)
        return bool(user and org_service.get_membership(db, org_id, user.id))
    finally:
        db.close()


@router.websocket("/ws/organizations/{org_id}")
async def org_telemetry_ws(
    websocket: WebSocket,
    org_id: uuid.UUID,
    token: str = Query(...),
):
    if not _authorize(org_id, token):
        await websocket.close(code=status.WS_1008_POLICY_VIOLATION)
        return

    # Subscribe BEFORE accept so no message is missed between connect and listen.
    r = aioredis.from_url(settings.REDIS_URL)
    pubsub = r.pubsub()
    await pubsub.subscribe(org_channel(org_id))
    await websocket.accept()

    async def relay():
        async for message in pubsub.listen():
            if message.get("type") == "message":
                data = message["data"]
                await websocket.send_text(data.decode() if isinstance(data, bytes) else data)

    async def watch_close():
        # Completes (raises WebSocketDisconnect) when the client goes away.
        while True:
            await websocket.receive_text()

    relay_task = asyncio.create_task(relay())
    watch_task = asyncio.create_task(watch_close())
    try:
        done, pending = await asyncio.wait(
            {relay_task, watch_task}, return_when=asyncio.FIRST_COMPLETED
        )
        for t in pending:
            t.cancel()
    except WebSocketDisconnect:
        pass
    finally:
        await pubsub.unsubscribe(org_channel(org_id))
        await pubsub.aclose()
        await r.aclose()
