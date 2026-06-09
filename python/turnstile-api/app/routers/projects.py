import uuid

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from ..deps import get_current_user, get_db
from ..models.user import User
from ..schemas.project import ProjectCreate, ProjectOut
from ..schemas.telemetry import (
    DeploymentCreate,
    DeploymentCreated,
    DeploymentOut,
    SessionOut,
    SummaryOut,
)
from ..services import deployment_service, org_service, project_service, telemetry_service

router = APIRouter(prefix="/organizations/{org_id}/projects", tags=["projects"])


def _require_org(db: Session, org_id: uuid.UUID, user: User):
    if org_service.get_organization(db, org_id) is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, detail="organization not found")
    org_service.require_member(db, org_id, user)


@router.post("", response_model=ProjectOut, status_code=status.HTTP_201_CREATED)
def create_project(
    org_id: uuid.UUID,
    data: ProjectCreate,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    org_service.require_admin(db, org_id, user)
    return ProjectOut.model_validate(project_service.create_project(db, org_id, data))


@router.get("", response_model=list[ProjectOut])
def list_projects(
    org_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    return [ProjectOut.model_validate(p) for p in project_service.list_for_org(db, org_id)]


@router.get("/{project_id}", response_model=ProjectOut)
def get_project(
    org_id: uuid.UUID,
    project_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    return ProjectOut.model_validate(project_service.get_project(db, org_id, project_id))


@router.post(
    "/{project_id}/deployments",
    response_model=DeploymentCreated,
    status_code=status.HTTP_201_CREATED,
)
def create_deployment(
    org_id: uuid.UUID,
    project_id: uuid.UUID,
    data: DeploymentCreate,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    org_service.require_admin(db, org_id, user)  # only admin/owner provision keys
    project_service.get_project(db, org_id, project_id)  # verify it belongs to the org
    deployment, plaintext = deployment_service.create_deployment(
        db, org_id, data.name, project_id=project_id
    )
    base = DeploymentOut.model_validate(deployment)
    return DeploymentCreated(**base.model_dump(), ingest_key=plaintext)  # key shown once


@router.get("/{project_id}/deployments", response_model=list[DeploymentOut])
def list_project_deployments(
    org_id: uuid.UUID,
    project_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    project_service.get_project(db, org_id, project_id)
    return [
        DeploymentOut.model_validate(d) for d in deployment_service.list_for_project(db, project_id)
    ]


@router.get("/{project_id}/summary", response_model=SummaryOut)
def project_summary(
    org_id: uuid.UUID,
    project_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    project_service.get_project(db, org_id, project_id)
    return SummaryOut(**telemetry_service.summary(db, org_id, project_id=project_id))


@router.get("/{project_id}/sessions", response_model=list[SessionOut])
def project_sessions(
    org_id: uuid.UUID,
    project_id: uuid.UUID,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    _require_org(db, org_id, user)
    project_service.get_project(db, org_id, project_id)
    return [
        SessionOut.model_validate(s)
        for s in telemetry_service.list_sessions(db, org_id, project_id=project_id)
    ]
