import uuid
from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, String
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import Mapped, mapped_column

from ..database import TimestampedBase


class Deployment(TimestampedBase):
    """A Turnstile Go container instance belonging to an organization. The
    container is configured with this deployment's ingest key; telemetry it posts
    attributes to this deployment/org. The raw key is shown once on creation and
    only its sha256 hash is stored."""

    __tablename__ = "deployments"

    organization_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("organizations.id"), index=True, nullable=False
    )
    project_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("projects.id"), index=True, nullable=True
    )
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    ingest_key_hash: Mapped[str] = mapped_column(
        String(64), unique=True, index=True, nullable=False
    )
    ingest_key_prefix: Mapped[str] = mapped_column(String(16), nullable=False)
    last_seen_at: Mapped[datetime | None] = mapped_column(
        DateTime(timezone=True), nullable=True
    )
