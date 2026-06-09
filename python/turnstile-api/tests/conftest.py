import pytest
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

import app.models  # noqa: F401  (register all models on Base.metadata)
from app.config import settings
from app.database import Base
from app.deps import get_db
from app.main import app

# Guard: never run the destructive test fixtures against a non-test database.
assert settings.DB_NAME.endswith("_test"), (
    f"refusing to run tests against {settings.DB_NAME!r}; run via `just test` "
    "(which sets DB_NAME=..._test)"
)

engine = create_engine(settings.sqlalchemy_url)
TestSession = sessionmaker(bind=engine, autoflush=False, autocommit=False)


@pytest.fixture(scope="session", autouse=True)
def _schema():
    Base.metadata.drop_all(engine)
    Base.metadata.create_all(engine)
    yield
    Base.metadata.drop_all(engine)


@pytest.fixture(autouse=True)
def _clean():
    """Truncate all tables after each test for isolation (services commit, so we
    can't rely on transaction rollback)."""
    yield
    with engine.begin() as conn:
        for table in reversed(Base.metadata.sorted_tables):
            conn.execute(table.delete())


def _override_get_db():
    s = TestSession()
    try:
        yield s
    finally:
        s.close()


@pytest.fixture
def db():
    s = TestSession()
    try:
        yield s
    finally:
        s.close()


@pytest.fixture
def client():
    app.dependency_overrides[get_db] = _override_get_db
    with TestClient(app) as c:
        yield c
    app.dependency_overrides.clear()
