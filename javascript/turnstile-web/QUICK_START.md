# Quick Start — Turnstile Web

The Next.js dashboard + account UI. It's a pure client of the API, so **start the API first**.

> **Running the whole stack end-to-end** (your app → Turnstile → OpenRouter → metrics here)? See [`../../docs/RUNNING_LOCALLY.md`](../../docs/RUNNING_LOCALLY.md).

## Prerequisites

- **Node 18+** (Node 25 used in dev) and **npm**.
- **just** (`brew install just`).
- The **API running** on `http://localhost:8000` (see `../../python/turnstile-api/QUICK_START.md`).

## Run it

```sh
cd javascript/turnstile-web
cp .env.example .env.local     # set NEXT_PUBLIC_API_URL if the API isn't on :8000
just install                   # npm install
just up                        # → http://localhost:3000
```

Open `http://localhost:3000`, register an account, and you're in.

## What to fill in `.env.local`

- `NEXT_PUBLIC_API_URL` — the API base URL. Default `http://localhost:8000` works for local dev.

## How it connects

All data (auth, orgs, telemetry) comes from the API over HTTP with a JWT (stored in `localStorage`). It never talks to Postgres or the Go container directly.

To wire up a Go container from the UI: create an **organization → a deployment**, then copy the **ingest key** shown once and put it in the Go container's `.env` (`TURNSTILE_INGEST_KEY`).

## Common commands

```sh
just build         # production build
just check         # lint + tsc --noEmit
just test          # Playwright E2E (API + web must be running)
```

See [`docs/FLOWS.md`](docs/FLOWS.md) for the page flows.
