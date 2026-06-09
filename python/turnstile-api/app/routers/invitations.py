from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from ..deps import get_current_user, get_db
from ..models.user import User
from ..schemas.organization import AcceptInvitation, MemberOut
from ..services import invitation_service

router = APIRouter(prefix="/invitations", tags=["invitations"])


@router.post("/accept", response_model=MemberOut)
def accept_invitation(
    data: AcceptInvitation,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    """Accept a pending invitation by token (the authenticated user's email must
    match the invitation). Returns the resulting membership."""
    _invitation, membership = invitation_service.accept_by_token(db, user, data.token)
    return MemberOut(
        id=membership.id,
        user_id=membership.user_id,
        organization_id=membership.organization_id,
        role=membership.role,
        created_at=membership.created_at,
        first_name=user.first_name,
        last_name=user.last_name,
        email=user.email,
    )
