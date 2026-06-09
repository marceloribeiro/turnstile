import uuid

from fastapi import HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from ..models.user import User
from ..schemas.user import UserCreate
from ..security import hash_password, verify_password


def get_by_email(db: Session, email: str) -> User | None:
    return db.execute(
        select(User).where(User.email == email, User.deleted_at.is_(None))
    ).scalar_one_or_none()


def get_by_id(db: Session, user_id: uuid.UUID) -> User | None:
    return db.execute(
        select(User).where(User.id == user_id, User.deleted_at.is_(None))
    ).scalar_one_or_none()


def create_user(db: Session, data: UserCreate) -> User:
    if get_by_email(db, str(data.email)):
        raise HTTPException(status.HTTP_409_CONFLICT, detail="email already registered")
    user = User(
        first_name=data.first_name,
        last_name=data.last_name,
        email=str(data.email),
        password_hash=hash_password(data.password),
    )
    db.add(user)
    db.commit()
    db.refresh(user)
    # Rule 1: auto-accept any pending organization invitations for this email.
    # Imported lazily to avoid an import cycle (invitation_service → org_service).
    from . import invitation_service

    invitation_service.auto_accept_on_signup(db, user)
    return user


def authenticate(db: Session, email: str, password: str) -> User | None:
    user = get_by_email(db, email)
    if not user or not verify_password(password, user.password_hash):
        return None
    return user
