import uuid

from fastapi import HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from ..models.organization import Organization
from ..models.organization_member import (
    ROLE_ADMIN,
    ROLE_OWNER,
    ROLE_RANK,
    OrganizationMember,
)
from ..models.user import User
from ..schemas.organization import OrganizationCreate


def get_organization(db: Session, org_id: uuid.UUID) -> Organization | None:
    return db.execute(
        select(Organization).where(
            Organization.id == org_id, Organization.deleted_at.is_(None)
        )
    ).scalar_one_or_none()


def get_membership(db: Session, org_id: uuid.UUID, user_id: uuid.UUID) -> OrganizationMember | None:
    return db.execute(
        select(OrganizationMember).where(
            OrganizationMember.organization_id == org_id,
            OrganizationMember.user_id == user_id,
            OrganizationMember.deleted_at.is_(None),
        )
    ).scalar_one_or_none()


def add_member(
    db: Session, org_id: uuid.UUID, user_id: uuid.UUID, role: str
) -> OrganizationMember:
    """Idempotent: returns the existing membership if present, else creates one."""
    existing = get_membership(db, org_id, user_id)
    if existing:
        return existing
    member = OrganizationMember(organization_id=org_id, user_id=user_id, role=role)
    db.add(member)
    db.commit()
    db.refresh(member)
    return member


def create_organization(db: Session, user: User, data: OrganizationCreate) -> Organization:
    org = Organization(name=data.name, website_url=data.website_url)
    db.add(org)
    db.commit()
    db.refresh(org)
    # Rule 2: the creator becomes the owner.
    add_member(db, org.id, user.id, ROLE_OWNER)
    return org


def list_for_user(db: Session, user_id: uuid.UUID) -> list[Organization]:
    return list(
        db.execute(
            select(Organization)
            .join(OrganizationMember, OrganizationMember.organization_id == Organization.id)
            .where(
                OrganizationMember.user_id == user_id,
                OrganizationMember.deleted_at.is_(None),
                Organization.deleted_at.is_(None),
            )
            .order_by(Organization.created_at.desc())
        ).scalars()
    )


def list_members(db: Session, org_id: uuid.UUID) -> list[OrganizationMember]:
    return list(
        db.execute(
            select(OrganizationMember).where(
                OrganizationMember.organization_id == org_id,
                OrganizationMember.deleted_at.is_(None),
            )
        ).scalars()
    )


def list_members_with_users(db: Session, org_id: uuid.UUID):
    """Returns (OrganizationMember, User) rows so the API can include each
    member's name and email, not just the membership row."""
    return db.execute(
        select(OrganizationMember, User)
        .join(User, User.id == OrganizationMember.user_id)
        .where(
            OrganizationMember.organization_id == org_id,
            OrganizationMember.deleted_at.is_(None),
            User.deleted_at.is_(None),
        )
        .order_by(OrganizationMember.created_at)
    ).all()


def require_member(db: Session, org_id: uuid.UUID, user: User) -> OrganizationMember:
    membership = get_membership(db, org_id, user.id)
    if membership is None:
        raise HTTPException(status.HTTP_403_FORBIDDEN, detail="not a member of this organization")
    return membership


def require_role(db: Session, org_id: uuid.UUID, user: User, minimum: str) -> OrganizationMember:
    membership = require_member(db, org_id, user)
    # ROLE_RANK.get(...): an unrecognized stored role is treated as lowest
    # privilege rather than raising KeyError (defensive — input is validated).
    if ROLE_RANK.get(membership.role, -1) < ROLE_RANK[minimum]:
        raise HTTPException(status.HTTP_403_FORBIDDEN, detail=f"requires {minimum} role")
    return membership


def require_admin(db: Session, org_id: uuid.UUID, user: User) -> OrganizationMember:
    return require_role(db, org_id, user, ROLE_ADMIN)
