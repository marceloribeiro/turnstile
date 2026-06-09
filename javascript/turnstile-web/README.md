# Turnstile — Web (Next.js)

The dashboard and account UI. A glassmorphic, Apple-vibrancy web app for **auth**, **organization management** (members, invitations, deployments), and the **dashboard** — with **dollars-prevented** as the hero metric. Talks to the Turnstile API over HTTP/JWT.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`docs/FLOWS.md`](docs/FLOWS.md).

## Stack

Next.js 16 (App Router) · React 19 · Tailwind CSS v4 · a hand-rolled **glass design system** (no component lib) · Playwright (E2E).

## Quickstart

```sh
npm install
echo 'NEXT_PUBLIC_API_URL=http://localhost:8000' > .env.local
npm run dev            # http://localhost:3000  (API must be running)
```

## Pages

| Route | Purpose |
|-------|---------|
| `/` | redirect → `/dashboard` or `/login` |
| `/login`, `/register` | auth (glass card) |
| `/dashboard` | list organizations + create |
| `/organizations/[id]` | **Overview** tab — dollars-prevented hero + live sessions table |
| `/organizations/[id]/members` | **Members** tab — members, pending invitations, invite form |
| `/organizations/[id]/projects` | **Projects** tab — list/create projects |
| `/organizations/[id]/projects/[projectId]` | project detail — hero + sessions + deployments and mint ingest key (with `.env` snippet) |
| `/invitations/accept?token=…` | accept an invitation |

## Layout

```
app/
  layout.tsx              root — gradient backdrop + orbs, <AuthProvider>
  globals.css             the glass design system (mesh gradient + .glass utilities)
  page.tsx                redirect
  login/ register/        auth pages
  dashboard/              org list + create
  organizations/[id]/     the org detail page (hero, sessions, members, invite, deployments)
  invitations/accept/     accept flow (Suspense-wrapped useSearchParams)
components/
  ui.tsx                  GlassCard, Button, Input, Field, Stat, Spinner, usd()
  brand.tsx               gradient shield logo
  auth-layout.tsx         centered glass card for auth
  app-shell.tsx           top bar (brand + user + sign out)
lib/
  api.ts                  fetch wrapper (base URL + JWT + ApiError)
  auth-context.tsx        AuthProvider, useAuth, useRequireAuth (token in localStorage)
  use-telemetry-socket.ts WebSocket hook — live org dashboard updates (Redis pub/sub on the API)
  types.ts                shared API types
tests/specs/              Playwright E2E (auth, auth-guards)
scripts/shots.mjs         screenshot helper
```

## Develop

```sh
npm run build             # production build
npx tsc --noEmit          # typecheck
npx playwright test       # E2E (servers must be running)
```

> **Next 16 note:** dynamic-route `params` is a `Promise`. In these client components we use the `useParams()` / `useSearchParams()` hooks (the latter inside a `<Suspense>` boundary). Consult `node_modules/next/dist/docs/` before writing routing code — Next 16 differs from older releases.
