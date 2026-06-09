"""Direct service-layer tests (the routers exercise these too, but a few
behaviours are easier to assert against the service API)."""

import uuid

import pytest
from fastapi import HTTPException

from app.models.organization_member import OrganizationMember
from app.models.user import User
from app.schemas.organization import OrganizationCreate
from app.security import hash_password
from app.services import deployment_service, invitation_service, org_service


def _make_user(db, email="svc@example.com") -> User:
    user = User(
        first_name="S", last_name="V", email=email, password_hash=hash_password("pw123456")
    )
    db.add(user)
    db.commit()
    db.refresh(user)
    return user


def test_ingest_key_hash_is_deterministic_sha256_and_prefix_only(db):
    org = org_service.create_organization(db, _make_user(db), OrganizationCreate(name="K"))
    deployment, plaintext = deployment_service.create_deployment(db, org.id, "d")

    assert plaintext.startswith("ts_")
    # Plaintext is never persisted; only its sha256 + a short display prefix.
    assert deployment.ingest_key_hash == deployment_service.hash_key(plaintext)
    assert deployment.ingest_key_hash != plaintext
    assert len(deployment.ingest_key_hash) == 64
    assert deployment.ingest_key_prefix == plaintext[:10]

    # Lookup round-trips by plaintext; a wrong key resolves to nothing.
    assert deployment_service.get_by_ingest_key(db, plaintext).id == deployment.id
    assert deployment_service.get_by_ingest_key(db, "ts_wrong") is None


def test_generated_keys_are_unique_high_entropy(db):
    keys = {deployment_service._generate_key() for _ in range(200)}
    assert len(keys) == 200


def test_require_role_treats_unknown_stored_role_as_lowest(db):
    # Defense in depth: even if a bogus role somehow lands in the DB, an admin
    # check must 403 rather than raise KeyError -> 500.
    user = _make_user(db, "bogus@example.com")
    org = org_service.create_organization(db, user, OrganizationCreate(name="B"))
    member = db.get(OrganizationMember, org_service.get_membership(db, org.id, user.id).id)
    member.role = "superadmin"
    db.commit()

    with pytest.raises(HTTPException) as exc:
        org_service.require_admin(db, org.id, user)
    assert exc.value.status_code == 403


def test_add_member_is_idempotent(db):
    user = _make_user(db, "idem@example.com")
    org = org_service.create_organization(db, user, OrganizationCreate(name="I"))
    first = org_service.get_membership(db, org.id, user.id)
    again = org_service.add_member(db, org.id, user.id, "member")
    assert again.id == first.id  # no duplicate membership


def test_get_organization_ignores_missing(db):
    assert org_service.get_organization(db, uuid.uuid4()) is None


def test_invitation_email_escapes_org_name_in_html(db, monkeypatch):
    # A malicious org name must not break out of the HTML email body.
    captured: dict = {}

    def _capture(to, subject, html_body, text_body=None):
        captured["html_body"] = html_body
        return False

    monkeypatch.setattr(invitation_service.mailer, "send_email", _capture)

    inviter = _make_user(db, "inviter@example.com")
    org = org_service.create_organization(
        db, inviter, OrganizationCreate(name="<script>alert(1)</script>")
    )
    invitation_service.create_invitation(db, org, "guest@example.com", "member", inviter)

    html_body = captured["html_body"]
    assert "<script>alert(1)</script>" not in html_body
    assert "&lt;script&gt;alert(1)&lt;/script&gt;" in html_body
