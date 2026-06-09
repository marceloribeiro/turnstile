import uuid
from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, EmailStr

# Must match the keys of organization_member.ROLE_RANK.
Role = Literal["member", "admin", "owner"]


class OrganizationCreate(BaseModel):
    name: str
    website_url: str | None = None


class OrganizationOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: uuid.UUID
    name: str
    website_url: str | None
    created_at: datetime


class MemberOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: uuid.UUID
    user_id: uuid.UUID
    organization_id: uuid.UUID
    role: str
    created_at: datetime
    first_name: str
    last_name: str
    email: EmailStr


class InvitationCreate(BaseModel):
    email: EmailStr
    role: Role = "member"


class InvitationOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: uuid.UUID
    organization_id: uuid.UUID
    email: EmailStr
    role: str
    status: str
    created_at: datetime


class AcceptInvitation(BaseModel):
    token: str
