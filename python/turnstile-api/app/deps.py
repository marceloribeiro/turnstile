import secrets
import uuid
from collections.abc import Generator

import jwt
from fastapi import Depends, Header, HTTPException, status
from fastapi.security import (
    HTTPAuthorizationCredentials,
    HTTPBasic,
    HTTPBasicCredentials,
    HTTPBearer,
)
from sqlalchemy.orm import Session

from .config import settings
from .database import SessionLocal
from .models.deployment import Deployment
from .models.user import User
from .security import decode_access_token
from .services import deployment_service, user_service


def get_db() -> Generator[Session]:
    """Per-request SQLAlchemy session."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


_basic = HTTPBasic()


def docs_auth(credentials: HTTPBasicCredentials = Depends(_basic)) -> None:
    """HTTP basic auth guard for the Swagger /docs (builder convention)."""
    ok_user = secrets.compare_digest(credentials.username, settings.DOCS_USERNAME)
    ok_pass = secrets.compare_digest(credentials.password, settings.DOCS_PASSWORD)
    if not (ok_user and ok_pass):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="invalid docs credentials",
            headers={"WWW-Authenticate": "Basic"},
        )


_bearer = HTTPBearer(auto_error=True)


def get_current_user(
    credentials: HTTPAuthorizationCredentials = Depends(_bearer),
    db: Session = Depends(get_db),
) -> User:
    """Resolve the authenticated user from a JWT bearer token."""
    invalid = HTTPException(
        status_code=status.HTTP_401_UNAUTHORIZED,
        detail="invalid or expired token",
        headers={"WWW-Authenticate": "Bearer"},
    )
    try:
        payload = decode_access_token(credentials.credentials)
        user_id = uuid.UUID(payload["sub"])
    except (jwt.PyJWTError, KeyError, ValueError):
        raise invalid from None
    user = user_service.get_by_id(db, user_id)
    if user is None:
        raise invalid
    return user


def get_ingest_deployment(
    x_turnstile_ingest_key: str = Header(..., alias="X-Turnstile-Ingest-Key"),
    db: Session = Depends(get_db),
) -> Deployment:
    """Authenticate a Go container by its per-deployment ingest key."""
    deployment = deployment_service.get_by_ingest_key(db, x_turnstile_ingest_key)
    if deployment is None:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, detail="invalid ingest key")
    return deployment
