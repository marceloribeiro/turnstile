from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Environment-driven config (builder convention). Every value has a
    dev-friendly default so the app boots locally without a .env; production
    supplies real values via the deployed .env."""

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    # PostgreSQL (native, no Docker — matches the workspace convention).
    DB_USER: str = "marcelo"
    DB_PASS: str = ""
    DB_HOST: str = "localhost"
    DB_PORT: int = 5432
    DB_NAME: str = "turnstile_api_dev"
    # Full URL override (used by alembic against the *_test DB, or in prod).
    DATABASE_URL: str | None = None

    # Auth (JWT).
    JWT_SECRET: str = "dev-insecure-change-me"
    JWT_ALGORITHM: str = "HS256"
    JWT_EXPIRE_MINUTES: int = 60 * 24 * 7  # 7 days

    # Swagger /docs HTTP basic auth (builder convention).
    DOCS_USERNAME: str = "turnstile"
    DOCS_PASSWORD: str = "turnstile"

    # Comma-separated CORS origins. Defaults to the local dashboard; production
    # MUST set the real dashboard origin(s) (a wildcard "*" allows all callers).
    CORS_ORIGINS: str = "http://localhost:3000"

    # Redis — pub/sub backbone for live dashboard WebSocket updates.
    REDIS_URL: str = "redis://localhost:6379/0"

    # Base URL of the web app — used to build invitation accept links.
    APP_BASE_URL: str = "http://localhost:3000"

    # Postmark — transactional email for organization invitations. Blank token
    # disables sending (the invitation is still created with its token).
    POSTMARK_API_TOKEN: str = ""
    POSTMARK_FROM: str = "no-reply@turnstile.dev"
    POSTMARK_MESSAGE_STREAM: str = "outbound"

    @property
    def email_enabled(self) -> bool:
        return bool(self.POSTMARK_API_TOKEN)

    @property
    def sqlalchemy_url(self) -> str:
        if self.DATABASE_URL:
            return self.DATABASE_URL
        auth = self.DB_USER if not self.DB_PASS else f"{self.DB_USER}:{self.DB_PASS}"
        return f"postgresql+psycopg://{auth}@{self.DB_HOST}:{self.DB_PORT}/{self.DB_NAME}"


settings = Settings()
