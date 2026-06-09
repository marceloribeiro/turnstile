import uuid
from datetime import datetime

from pydantic import BaseModel, ConfigDict


class DeploymentCreate(BaseModel):
    name: str


class DeploymentOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: uuid.UUID
    organization_id: uuid.UUID
    project_id: uuid.UUID | None
    name: str
    ingest_key_prefix: str
    last_seen_at: datetime | None
    created_at: datetime


class DeploymentCreated(DeploymentOut):
    """Returned once on creation — includes the plaintext ingest key."""

    ingest_key: str


class IngestSession(BaseModel):
    """One session aggregate, field-aligned with Go's meter.Record JSON so the Go
    client can post its snapshot verbatim. Unknown fields (e.g. last_cost) are
    ignored."""

    id: str  # the session key
    source: str = ""
    user: str = ""
    key_fp: str = ""
    model: str = ""
    requests: int = 0
    prompt_tokens: int = 0
    completion_tokens: int = 0
    cost: float = 0.0
    blocks: int = 0
    prevented: float = 0.0


class IngestBatch(BaseModel):
    sessions: list[IngestSession]


class IngestResult(BaseModel):
    upserted: int


class SessionOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: uuid.UUID
    deployment_id: uuid.UUID
    project_id: uuid.UUID | None
    session_key: str
    source: str
    user_label: str | None
    model: str | None
    requests: int
    prompt_tokens: int
    completion_tokens: int
    cost: float
    blocks: int
    prevented: float
    last_seen_at: datetime


class SummaryOut(BaseModel):
    sessions: int
    requests: int
    total_cost: float
    blocks: int
    dollars_prevented: float
