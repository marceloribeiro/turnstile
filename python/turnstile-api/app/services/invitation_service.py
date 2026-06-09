import datetime as dt
import html
import secrets
import uuid

from fastapi import HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from .. import mailer
from ..config import settings
from ..models.organization import Organization
from ..models.organization_invitation import (
    STATUS_ACCEPTED,
    STATUS_PENDING,
    OrganizationInvitation,
)
from ..models.organization_member import OrganizationMember
from ..models.user import User
from . import org_service


def _pending_for_email(db: Session, email: str) -> list[OrganizationInvitation]:
    return list(
        db.execute(
            select(OrganizationInvitation).where(
                OrganizationInvitation.email == email,
                OrganizationInvitation.status == STATUS_PENDING,
                OrganizationInvitation.deleted_at.is_(None),
            )
        ).scalars()
    )


def create_invitation(
    db: Session,
    org: Organization,
    email: str,
    role: str,
    invited_by: User,
) -> OrganizationInvitation:
    invitation = OrganizationInvitation(
        organization_id=org.id,
        email=email,
        role=role,
        token=secrets.token_urlsafe(32),
        invited_by_user_id=invited_by.id,
    )
    db.add(invitation)
    db.commit()
    db.refresh(invitation)
    _send_email(org, invitation)
    return invitation


def _send_email(org: Organization, invitation: OrganizationInvitation) -> None:
    accept_url = f"{settings.APP_BASE_URL}/invitations/accept?token={invitation.token}"
    # User-controlled org name is interpolated into HTML; escape to prevent
    # injection into the rendered email body.
    org_name = html.escape(org.name)
    role = html.escape(invitation.role)
    mailer.send_email(
        to=invitation.email,
        subject=f"You're invited to {org.name} on Turnstile",
        html_body=(
            f"<p>You've been invited to join <strong>{org_name}</strong> on Turnstile "
            f"as a {role}.</p>"
            f'<p><a href="{html.escape(accept_url)}">Accept the invitation</a></p>'
        ),
        text_body=f"Join {org.name} on Turnstile ({invitation.role}): {accept_url}",
    )


def list_pending(db: Session, org_id: uuid.UUID) -> list[OrganizationInvitation]:
    return list(
        db.execute(
            select(OrganizationInvitation).where(
                OrganizationInvitation.organization_id == org_id,
                OrganizationInvitation.status == STATUS_PENDING,
                OrganizationInvitation.deleted_at.is_(None),
            )
        ).scalars()
    )


def _accept(
    db: Session, invitation: OrganizationInvitation, user: User
) -> OrganizationMember:
    member = org_service.add_member(
        db, invitation.organization_id, user.id, invitation.role
    )
    invitation.status = STATUS_ACCEPTED
    invitation.accepted_at = dt.datetime.now(dt.UTC)
    db.commit()
    return member


def accept_by_token(
    db: Session, user: User, token: str
) -> tuple[OrganizationInvitation, OrganizationMember]:
    invitation = db.execute(
        select(OrganizationInvitation).where(
            OrganizationInvitation.token == token,
            OrganizationInvitation.deleted_at.is_(None),
        )
    ).scalar_one_or_none()
    if invitation is None or invitation.status != STATUS_PENDING:
        raise HTTPException(
            status.HTTP_404_NOT_FOUND, detail="invitation not found or already used"
        )
    if invitation.email.lower() != user.email.lower():
        raise HTTPException(
            status.HTTP_403_FORBIDDEN, detail="this invitation is for a different email"
        )
    member = _accept(db, invitation, user)
    return invitation, member


def auto_accept_on_signup(db: Session, user: User) -> int:
    """ORGANIZATION_BASED.md rule 1: on signup, auto-accept all pending
    invitations addressed to this user's email. (We honor each invitation's role
    rather than forcing 'member', which matches the invite's intent.)"""
    accepted = 0
    for invitation in _pending_for_email(db, user.email):
        _accept(db, invitation, user)
        accepted += 1
    return accepted
