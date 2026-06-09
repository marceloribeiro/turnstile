"""Transactional email via the Postmark HTTP API. A blank POSTMARK_API_TOKEN
disables sending (the caller still proceeds — e.g. an invitation is created with
its token even if no email goes out)."""

import logging

import httpx

from .config import settings

log = logging.getLogger("turnstile.mailer")


def send_email(to: str, subject: str, html_body: str, text_body: str | None = None) -> bool:
    if not settings.email_enabled:
        log.info("email disabled (no POSTMARK_API_TOKEN) — skipping send to %s", to)
        return False
    try:
        resp = httpx.post(
            "https://api.postmarkapp.com/email",
            headers={
                "X-Postmark-Server-Token": settings.POSTMARK_API_TOKEN,
                "Accept": "application/json",
            },
            json={
                "From": settings.POSTMARK_FROM,
                "To": to,
                "Subject": subject,
                "HtmlBody": html_body,
                "TextBody": text_body or "",
                "MessageStream": settings.POSTMARK_MESSAGE_STREAM,
            },
            timeout=10,
        )
        return resp.status_code == 200
    except httpx.HTTPError as exc:
        log.warning("postmark send failed for %s: %s", to, exc)
        return False
