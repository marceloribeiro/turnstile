from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from ..deps import get_current_user, get_db
from ..models.user import User
from ..schemas.telemetry import UsageOut
from ..schemas.user import AuthResponse, UserCreate, UserLogin, UserOut
from ..security import create_access_token
from ..services import telemetry_service, user_service

router = APIRouter(tags=["auth"])


@router.post("/register", response_model=AuthResponse, status_code=status.HTTP_201_CREATED)
def register(data: UserCreate, db: Session = Depends(get_db)):
    user = user_service.create_user(db, data)
    return AuthResponse(user=UserOut.model_validate(user), token=create_access_token(str(user.id)))


@router.post("/login", response_model=AuthResponse)
def login(data: UserLogin, db: Session = Depends(get_db)):
    user = user_service.authenticate(db, str(data.email), data.password)
    if user is None:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, detail="invalid credentials")
    return AuthResponse(user=UserOut.model_validate(user), token=create_access_token(str(user.id)))


@router.get("/me", response_model=UserOut)
def me(current_user: User = Depends(get_current_user)):
    return UserOut.model_validate(current_user)


@router.get("/me/usage", response_model=UsageOut)
def my_usage(
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    """Cross-org spend for the current user, broken down by model."""
    return UsageOut(**telemetry_service.usage_for_user(db, current_user.id))
