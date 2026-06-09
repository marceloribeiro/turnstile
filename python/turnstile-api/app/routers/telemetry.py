import uuid

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from ..deps import get_current_user, get_db
from ..models.user import User
from ..schemas.telemetry import DeploymentOut, SessionOut, SummaryOut
from ..services import deployment_service, org_service, telemetry_service

router = APIRouter(prefix="/organizations/{org_id}", tags=["telemetry"])


def _require_org(db: Session, org_id: uuid.UUID, user: User):
    if org_service.get_organization(db, org_id) is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, detail="organization not found")
    org_service.require_member(db, org_id, user)


@router.get("/deployments", response_model=list[DeploymentOut])
def list_deployments(
    org_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    """All deployments across the org (deployment creation now lives under a project)."""
    _require_org(db, org_id, user)
    return [DeploymentOut.model_validate(d) for d in deployment_service.list_for_org(db, org_id)]


@router.get("/sessions", response_model=list[SessionOut])
def list_sessions(
    org_id: uuid.UUID,
    project_id: uuid.UUID | None = None,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    return [
        SessionOut.model_validate(s)
        for s in telemetry_service.list_sessions(db, org_id, project_id=project_id)
    ]


@router.get("/summary", response_model=SummaryOut)
def org_summary(
    org_id: uuid.UUID,
    project_id: uuid.UUID | None = None,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    return SummaryOut(**telemetry_service.summary(db, org_id, project_id=project_id))
