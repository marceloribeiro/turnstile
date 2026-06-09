from sqlalchemy import String
from sqlalchemy.orm import Mapped, mapped_column

from ..database import TimestampedBase


class Organization(TimestampedBase):
    __tablename__ = "organizations"

    name: Mapped[str] = mapped_column(String(255), nullable=False)
    website_url: Mapped[str | None] = mapped_column(String(512), nullable=True)
