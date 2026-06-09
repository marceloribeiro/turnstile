from fastapi import Depends, FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .config import settings
from .deps import docs_auth
from .routers import (
    auth,
    health,
    ingest,
    invitations,
    organizations,
    projects,
    telemetry,
    ws,
)

# /docs and /openapi.json are gated behind HTTP basic auth (builder convention).
app = FastAPI(
    title="Turnstile API",
    version="0.1.0",
    docs_url=None,
    redoc_url=None,
    openapi_url=None,
)

origins = [o.strip() for o in settings.CORS_ORIGINS.split(",") if o.strip()] or ["*"]
# No credentials: the API authenticates via bearer tokens, not cookies, and
# allow_credentials=True is invalid combined with a wildcard origin.
app.add_middleware(
    CORSMiddleware,
    allow_origins=origins,
    allow_credentials=False,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(health.router)
app.include_router(auth.router)
app.include_router(organizations.router)
app.include_router(invitations.router)
app.include_router(projects.router)
app.include_router(telemetry.router)
app.include_router(ingest.router)
app.include_router(ws.router)


# Auth-gated OpenAPI + Swagger UI.
from fastapi.openapi.docs import get_swagger_ui_html  # noqa: E402
from fastapi.openapi.utils import get_openapi  # noqa: E402


@app.get("/openapi.json", include_in_schema=False)
def openapi(_: None = Depends(docs_auth)):
    return get_openapi(title=app.title, version=app.version, routes=app.routes)


@app.get("/docs", include_in_schema=False)
def docs(_: None = Depends(docs_auth)):
    return get_swagger_ui_html(openapi_url="/openapi.json", title=f"{app.title} docs")
