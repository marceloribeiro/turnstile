def register(client, email, password="pw123456", first="Test", last="User"):
    r = client.post(
        "/register",
        json={"first_name": first, "last_name": last, "email": email, "password": password},
    )
    assert r.status_code == 201, r.text
    return r.json()["token"]


def auth(token):
    return {"Authorization": f"Bearer {token}"}


def test_create_org_makes_creator_owner(client):
    tok = register(client, "owner@example.com")
    r = client.post("/organizations", json={"name": "Acme", "website_url": "https://acme.co"}, headers=auth(tok))
    assert r.status_code == 201
    org_id = r.json()["id"]

    # creator is listed as a member with role owner
    members = client.get(f"/organizations/{org_id}/members", headers=auth(tok)).json()
    assert len(members) == 1
    assert members[0]["role"] == "owner"

    # and the org shows up in their list
    mine = client.get("/organizations", headers=auth(tok)).json()
    assert [o["id"] for o in mine] == [org_id]


def test_non_member_cannot_view_org(client):
    owner = register(client, "o2@example.com")
    org_id = client.post("/organizations", json={"name": "Private"}, headers=auth(owner)).json()["id"]

    outsider = register(client, "outsider@example.com")
    r = client.get(f"/organizations/{org_id}", headers=auth(outsider))
    assert r.status_code == 403


def test_invite_then_accept_by_token(client):
    owner = register(client, "boss@example.com")
    org_id = client.post("/organizations", json={"name": "TeamCo"}, headers=auth(owner)).json()["id"]

    # The invitee already has an account BEFORE the invitation, so rule-1
    # auto-accept does NOT fire — they accept explicitly via the token/link.
    newbie = register(client, "newbie@example.com")

    inv = client.post(
        f"/organizations/{org_id}/invitations",
        json={"email": "newbie@example.com", "role": "admin"},
        headers=auth(owner),
    )
    assert inv.status_code == 201
    pending = client.get(f"/organizations/{org_id}/invitations", headers=auth(owner)).json()
    assert len(pending) == 1 and pending[0]["email"] == "newbie@example.com"

    token = _invitation_token(client, org_id, owner, "newbie@example.com")
    acc = client.post("/invitations/accept", json={"token": token}, headers=auth(newbie))
    assert acc.status_code == 200
    assert acc.json()["role"] == "admin"

    members = client.get(f"/organizations/{org_id}/members", headers=auth(owner)).json()
    assert {m["role"] for m in members} == {"owner", "admin"}


def test_accept_wrong_email_forbidden(client):
    owner = register(client, "boss2@example.com")
    org_id = client.post("/organizations", json={"name": "Strict"}, headers=auth(owner)).json()["id"]
    client.post(
        f"/organizations/{org_id}/invitations",
        json={"email": "intended@example.com", "role": "member"},
        headers=auth(owner),
    )
    token = _invitation_token(client, org_id, owner, "intended@example.com")
    # a different, already-registered user must not be able to claim it
    interloper = register(client, "interloper@example.com")
    r = client.post("/invitations/accept", json={"token": token}, headers=auth(interloper))
    assert r.status_code == 403


def test_auto_accept_on_signup(client):
    owner = register(client, "founder@example.com")
    org_id = client.post("/organizations", json={"name": "AutoCo"}, headers=auth(owner)).json()["id"]
    client.post(
        f"/organizations/{org_id}/invitations",
        json={"email": "future@example.com", "role": "member"},
        headers=auth(owner),
    )

    # the invited person signs up AFTER being invited → membership auto-created
    invitee = register(client, "future@example.com")
    mine = client.get("/organizations", headers=auth(invitee)).json()
    assert [o["id"] for o in mine] == [org_id]


def test_only_admin_can_invite(client):
    owner = register(client, "owner3@example.com")
    org_id = client.post("/organizations", json={"name": "RoleCo"}, headers=auth(owner)).json()["id"]
    # add a plain member via invite + auto-accept
    client.post(
        f"/organizations/{org_id}/invitations",
        json={"email": "plain@example.com", "role": "member"},
        headers=auth(owner),
    )
    member = register(client, "plain@example.com")
    # a member tries to invite → 403
    r = client.post(
        f"/organizations/{org_id}/invitations",
        json={"email": "another@example.com", "role": "member"},
        headers=auth(member),
    )
    assert r.status_code == 403


def test_invite_rejects_unknown_role(client):
    # An arbitrary role string must be rejected (422), not stored — otherwise a
    # membership with a bogus role crashes the ROLE_RANK lookup in require_role.
    owner = register(client, "roleval@example.com")
    org_id = client.post(
        "/organizations", json={"name": "ValCo"}, headers=auth(owner)
    ).json()["id"]
    r = client.post(
        f"/organizations/{org_id}/invitations",
        json={"email": "x@example.com", "role": "superadmin"},
        headers=auth(owner),
    )
    assert r.status_code == 422


# --- test helper: read an invitation token directly from the DB ----------
def _invitation_token(client, org_id, owner_token, email):
    from sqlalchemy import select

    from app.models.organization_invitation import OrganizationInvitation
    from tests.conftest import TestSession

    with TestSession() as s:
        inv = s.execute(
            select(OrganizationInvitation).where(OrganizationInvitation.email == email)
        ).scalar_one()
        return inv.token
