import uuid

from fastapi import HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from ..models.project import Project
from ..schemas.project import ProjectCreate


def create_project(db: Session, org_id: uuid.UUID, data: ProjectCreate) -> Project:
    project = Project(organization_id=org_id, name=data.name, description=data.description)
    db.add(project)
    db.commit()
    db.refresh(project)
    return project


def list_for_org(db: Session, org_id: uuid.UUID) -> list[Project]:
    return list(
        db.execute(
            select(Project)
            .where(Project.organization_id == org_id, Project.deleted_at.is_(None))
            .order_by(Project.created_at.desc())
        ).scalars()
    )


def get_project(db: Session, org_id: uuid.UUID, project_id: uuid.UUID) -> Project:
    project = db.execute(
        select(Project).where(
            Project.id == project_id,
            Project.organization_id == org_id,
            Project.deleted_at.is_(None),
        )
    ).scalar_one_or_none()
    if project is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, detail="project not found")
    return project
