import uuid
from datetime import datetime

from sqlalchemy import BigInteger, DateTime, Float, ForeignKey, String, UniqueConstraint
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import Mapped, mapped_column

from ..database import TimestampedBase


class TurnstileSession(TimestampedBase):
    """Durable per-session telemetry written by the Go container via the ingest
    endpoint. Aggregates only (hashes + counts + cost) — never raw prompts/keys."""

    __tablename__ = "turnstile_sessions"
    __table_args__ = (
        UniqueConstraint("deployment_id", "session_key", name="uq_turnstile_session"),
    )

    organization_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("organizations.id"), index=True, nullable=False
    )
    deployment_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("deployments.id"), index=True, nullable=False
    )
    project_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("projects.id"), index=True, nullable=True
    )
    session_key: Mapped[str] = mapped_column(String(128), index=True, nullable=False)
    source: Mapped[str] = mapped_column(String(20), default="", nullable=False)
    user_label: Mapped[str | None] = mapped_column(String(255), nullable=True)
    key_fp: Mapped[str | None] = mapped_column(String(64), nullable=True)
    model: Mapped[str | None] = mapped_column(String(128), nullable=True)

    requests: Mapped[int] = mapped_column(BigInteger, default=0, nullable=False)
    prompt_tokens: Mapped[int] = mapped_column(BigInteger, default=0, nullable=False)
    completion_tokens: Mapped[int] = mapped_column(BigInteger, default=0, nullable=False)
    cost: Mapped[float] = mapped_column(Float, default=0.0, nullable=False)
    blocks: Mapped[int] = mapped_column(BigInteger, default=0, nullable=False)
    prevented: Mapped[float] = mapped_column(Float, default=0.0, nullable=False)

    first_seen_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    last_seen_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
