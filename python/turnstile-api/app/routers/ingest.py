from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from .. import pubsub
from ..deps import get_db, get_ingest_deployment
from ..models.deployment import Deployment
from ..schemas.telemetry import IngestBatch, IngestResult
from ..services import telemetry_service

router = APIRouter(prefix="/ingest", tags=["ingest"])


@router.post("/sessions", response_model=IngestResult)
def ingest_sessions(
    batch: IngestBatch,
    db: Session = Depends(get_db),
    deployment: Deployment = Depends(get_ingest_deployment),
):
    """The Go container POSTs its session snapshot here, authed by its ingest key
    (X-Turnstile-Ingest-Key header). Aggregates are upserted, scoped to the
    deployment's organization."""
    n = telemetry_service.upsert_sessions(db, deployment, batch.sessions)
    # Notify any dashboards watching this org (live WebSocket update).
    pubsub.publish_telemetry(deployment.organization_id, upserted=n)
    return IngestResult(upserted=n)
