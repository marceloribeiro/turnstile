import uuid

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from ..deps import get_current_user, get_db
from ..models.user import User
from ..schemas.organization import (
    InvitationCreate,
    InvitationOut,
    MemberOut,
    OrganizationCreate,
    OrganizationOut,
)
from ..services import invitation_service, org_service

router = APIRouter(prefix="/organizations", tags=["organizations"])


def _get_org_or_404(db: Session, org_id: uuid.UUID):
    org = org_service.get_organization(db, org_id)
    if org is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, detail="organization not found")
    return org


@router.post("", response_model=OrganizationOut, status_code=status.HTTP_201_CREATED)
def create_organization(
    data: OrganizationCreate,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    org = org_service.create_organization(db, user, data)
    return OrganizationOut.model_validate(org)


@router.get("", response_model=list[OrganizationOut])
def list_my_organizations(
    db: Session = Depends(get_db), user: User = Depends(get_current_user)
):
    return [OrganizationOut.model_validate(o) for o in org_service.list_for_user(db, user.id)]


@router.get("/{org_id}", response_model=OrganizationOut)
def get_organization(
    org_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    org = _get_org_or_404(db, org_id)
    org_service.require_member(db, org_id, user)
    return OrganizationOut.model_validate(org)


@router.get("/{org_id}/members", response_model=list[MemberOut])
def list_members(
    org_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _get_org_or_404(db, org_id)
    org_service.require_member(db, org_id, user)
    return [
        MemberOut(
            id=m.id,
            user_id=m.user_id,
            organization_id=m.organization_id,
            role=m.role,
            created_at=m.created_at,
            first_name=u.first_name,
            last_name=u.last_name,
            email=u.email,
        )
        for m, u in org_service.list_members_with_users(db, org_id)
    ]


@router.post(
    "/{org_id}/invitations",
    response_model=InvitationOut,
    status_code=status.HTTP_201_CREATED,
)
def invite_member(
    org_id: uuid.UUID,
    data: InvitationCreate,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    org = _get_org_or_404(db, org_id)
    org_service.require_admin(db, org_id, user)  # only admin/owner may invite
    invitation = invitation_service.create_invitation(
        db, org, str(data.email), data.role, user
    )
    return InvitationOut.model_validate(invitation)


@router.get("/{org_id}/invitations", response_model=list[InvitationOut])
def list_invitations(
    org_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _get_org_or_404(db, org_id)
    org_service.require_admin(db, org_id, user)
    return [InvitationOut.model_validate(i) for i in invitation_service.list_pending(db, org_id)]
