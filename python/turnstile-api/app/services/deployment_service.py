import hashlib
import secrets
import uuid

from sqlalchemy import select
from sqlalchemy.orm import Session

from ..models.deployment import Deployment


def _generate_key() -> str:
    return "ts_" + secrets.token_urlsafe(32)


def hash_key(key: str) -> str:
    """sha256 — the ingest key is high-entropy, so a fast deterministic hash is
    appropriate (we need exact lookup; it's a bearer token, not a password)."""
    return hashlib.sha256(key.encode("utf-8")).hexdigest()


def create_deployment(
    db: Session, org_id: uuid.UUID, name: str, project_id: uuid.UUID | None = None
) -> tuple[Deployment, str]:
    """Returns (deployment, plaintext_key). The plaintext is shown once and never
    stored — only its hash and a display prefix are persisted."""
    plaintext = _generate_key()
    deployment = Deployment(
        organization_id=org_id,
        project_id=project_id,
        name=name,
        ingest_key_hash=hash_key(plaintext),
        ingest_key_prefix=plaintext[:10],
    )
    db.add(deployment)
    db.commit()
    db.refresh(deployment)
    return deployment, plaintext


def list_for_org(db: Session, org_id: uuid.UUID) -> list[Deployment]:
    return list(
        db.execute(
            select(Deployment)
            .where(Deployment.organization_id == org_id, Deployment.deleted_at.is_(None))
            .order_by(Deployment.created_at.desc())
        ).scalars()
    )


def list_for_project(db: Session, project_id: uuid.UUID) -> list[Deployment]:
    return list(
        db.execute(
            select(Deployment)
            .where(Deployment.project_id == project_id, Deployment.deleted_at.is_(None))
            .order_by(Deployment.created_at.desc())
        ).scalars()
    )


def get_by_ingest_key(db: Session, key: str) -> Deployment | None:
    return db.execute(
        select(Deployment).where(
            Deployment.ingest_key_hash == hash_key(key),
            Deployment.deleted_at.is_(None),
        )
    ).scalar_one_or_none()
