import datetime as dt
import uuid

from sqlalchemy import func, select
from sqlalchemy.dialects.postgresql import insert
from sqlalchemy.orm import Session

from ..models.deployment import Deployment
from ..models.organization_member import OrganizationMember
from ..models.turnstile_session import TurnstileSession
from ..schemas.telemetry import IngestSession


def upsert_sessions(
    db: Session, deployment: Deployment, sessions: list[IngestSession]
) -> int:
    """UPSERT each session aggregate by (deployment_id, session_key). Values are
    absolute (a snapshot from Go), so we overwrite rather than add.

    NOTE: Go's in-memory counters reset on container restart, so absolute
    overwrite can lose pre-restart history. Delta-based ingest is a later
    refinement; for now the durable record tracks the current container's view."""
    now = dt.datetime.now(dt.UTC)
    for s in sessions:
        # ON CONFLICT (deployment_id, session_key) does an atomic UPSERT, so
        # concurrent posts for the same session can't race into a unique-violation.
        values = {
            "organization_id": deployment.organization_id,
            "deployment_id": deployment.id,
            "project_id": deployment.project_id,
            "session_key": s.id,
            "source": s.source,
            "user_label": s.user or None,
            "key_fp": s.key_fp or None,
            "model": s.model or None,
            "requests": s.requests,
            "prompt_tokens": s.prompt_tokens,
            "completion_tokens": s.completion_tokens,
            "cost": s.cost,
            "blocks": s.blocks,
            "prevented": s.prevented,
            "first_seen_at": now,
            "last_seen_at": now,
        }
        stmt = insert(TurnstileSession).values(**values)
        stmt = stmt.on_conflict_do_update(
            constraint="uq_turnstile_session",
            set_={
                "project_id": stmt.excluded.project_id,
                "source": stmt.excluded.source,
                "user_label": stmt.excluded.user_label,
                "key_fp": stmt.excluded.key_fp,
                "model": stmt.excluded.model,
                "requests": stmt.excluded.requests,
                "prompt_tokens": stmt.excluded.prompt_tokens,
                "completion_tokens": stmt.excluded.completion_tokens,
                "cost": stmt.excluded.cost,
                "blocks": stmt.excluded.blocks,
                "prevented": stmt.excluded.prevented,
                "last_seen_at": stmt.excluded.last_seen_at,
                # A live container re-reporting revives a soft-deleted row.
                "deleted_at": None,
                "updated_at": now,
            },
        )
        db.execute(stmt)

    deployment.last_seen_at = now
    db.commit()
    return len(sessions)


def list_sessions(
    db: Session, org_id: uuid.UUID, project_id: uuid.UUID | None = None, limit: int = 200
) -> list[TurnstileSession]:
    stmt = (
        select(TurnstileSession)
        .where(
            TurnstileSession.organization_id == org_id,
            TurnstileSession.deleted_at.is_(None),
        )
        .order_by(TurnstileSession.cost.desc())
        .limit(limit)
    )
    if project_id is not None:
        stmt = stmt.where(TurnstileSession.project_id == project_id)
    return list(db.execute(stmt).scalars())


def summary(db: Session, org_id: uuid.UUID, project_id: uuid.UUID | None = None) -> dict:
    stmt = select(
        func.count(TurnstileSession.id),
        func.coalesce(func.sum(TurnstileSession.requests), 0),
        func.coalesce(func.sum(TurnstileSession.cost), 0.0),
        func.coalesce(func.sum(TurnstileSession.blocks), 0),
        func.coalesce(func.sum(TurnstileSession.prevented), 0.0),
    ).where(
        TurnstileSession.organization_id == org_id,
        TurnstileSession.deleted_at.is_(None),
    )
    if project_id is not None:
        stmt = stmt.where(TurnstileSession.project_id == project_id)
    row = db.execute(stmt).one()
    return {
        "sessions": int(row[0]),
        "requests": int(row[1]),
        "total_cost": float(row[2]),
        "blocks": int(row[3]),
        "dollars_prevented": float(row[4]),
    }


def usage_for_user(db: Session, user_id: uuid.UUID) -> dict:
    """Cross-org usage for one user, scoped to the orgs they belong to and broken
    down by model (highest spend first)."""
    member_orgs = (
        select(OrganizationMember.organization_id)
        .where(
            OrganizationMember.user_id == user_id,
            OrganizationMember.deleted_at.is_(None),
        )
        .scalar_subquery()
    )
    scope = (
        TurnstileSession.organization_id.in_(member_orgs),
        TurnstileSession.deleted_at.is_(None),
    )

    totals = db.execute(
        select(
            func.count(TurnstileSession.id),
            func.coalesce(func.sum(TurnstileSession.requests), 0),
            func.coalesce(func.sum(TurnstileSession.cost), 0.0),
            func.coalesce(func.sum(TurnstileSession.prevented), 0.0),
        ).where(*scope)
    ).one()

    model = func.coalesce(TurnstileSession.model, "unknown")
    rows = db.execute(
        select(
            model.label("model"),
            func.coalesce(func.sum(TurnstileSession.cost), 0.0),
            func.coalesce(func.sum(TurnstileSession.requests), 0),
            func.coalesce(func.sum(TurnstileSession.prompt_tokens), 0),
            func.coalesce(func.sum(TurnstileSession.completion_tokens), 0),
        )
        .where(*scope)
        .group_by(model)
        .order_by(func.sum(TurnstileSession.cost).desc())
    ).all()

    return {
        "sessions": int(totals[0]),
        "requests": int(totals[1]),
        "total_cost": float(totals[2]),
        "dollars_prevented": float(totals[3]),
        "by_model": [
            {
                "model": r[0],
                "cost": float(r[1]),
                "requests": int(r[2]),
                "prompt_tokens": int(r[3]),
                "completion_tokens": int(r[4]),
            }
            for r in rows
        ],
    }
