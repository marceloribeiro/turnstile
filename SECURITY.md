# Security Policy

## Reporting a vulnerability

Please report security issues privately rather than opening a public issue.
Use GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
("Report a vulnerability" under the repository's **Security** tab), or contact
the maintainer directly.

We will acknowledge your report as quickly as we can and keep you updated on the
fix. Please give us a reasonable window to address the issue before any public
disclosure.

## Design notes relevant to security

Turnstile sits in the path of LLM traffic, so it is designed to minimize what it
ever sees or stores:

- **No raw secrets at rest.** Provider API keys are passed through to the
  upstream untouched and are never logged or persisted — only a salted HMAC
  fingerprint is kept for session attribution.
- **No raw prompts at rest.** Prompt content is reduced to hashed anchors;
  only hashes and numeric aggregates leave the data-plane process.
- **Fail-open by default.** A fault in Turnstile forwards the request rather
  than blocking the customer's application (configurable to fail-closed).
- **Per-deployment ingest keys** authenticate the data plane to the control
  plane and are stored only as SHA-256 hashes.

## Operational guidance

- Always set strong, unique values for `JWT_SECRET`, the data-plane salt, and
  ingest keys in production. Never reuse the example values.
- Rotate any credential that may have been exposed.
