# Architecture — Turnstile Web

## Key tech

- **Next.js 16** (App Router) + **React 19** — client-rendered SPA-style pages that fetch from the API at runtime.
- **Tailwind CSS v4** — CSS-based config (`@import "tailwindcss"` + `@theme` in `globals.css`); no JS config file.
- **Glass design system** (hand-rolled, no component library) — chosen for full control of the Apple-vibrancy aesthetic.
- **Playwright** — E2E.

## The glass design system (`app/globals.css`)

- **Backdrop:** a fixed mesh of radial gradients (indigo / pink / sky / emerald) over near-black, plus two blurred `.orb` divs that slowly drift — depth without imagery.
- **`.glass` / `.glass-strong`:** translucent white + `backdrop-filter: blur() saturate()` + hairline border + layered shadow with an inner top highlight. The core frosted-card look.
- **`.glass-input`, `.btn`/`.btn-primary`/`.btn-ghost`, `.chip`, `.label`, `.muted`:** the rest of the kit. Primary buttons use an indigo→fuchsia gradient.
- System font stack (SF Pro / system-ui) for the native feel.

`components/ui.tsx` wraps these as React primitives (`GlassCard`, `Button`, `Input`, `Field`, `Stat`, `Spinner`, `CenterSpinner`, `ErrorText`, `usd()`).

## Components

| File | Role |
|------|------|
| `components/brand.tsx` | gradient shield + checkmark logo |
| `components/auth-layout.tsx` | centered glass card used by login/register/accept |
| `components/app-shell.tsx` | sticky top bar (brand, user email, sign out) for authed pages |
| `components/ui.tsx` | the glass primitive kit |

## Data & auth layer (`lib/`)

- **`api.ts`** — `api<T>(path, {method, body, token})`: prepends `NEXT_PUBLIC_API_URL`, sets `Content-Type` + `Authorization: Bearer`, throws a typed `ApiError` on non-2xx (surfaces the API's `detail`).
- **`auth-context.tsx`** — `<AuthProvider>` holds `{user, token}`; on mount it reads the token from `localStorage` and calls `/me`. `login`/`register` persist the token; `logout` clears it. `useRequireAuth()` redirects unauthenticated users to `/login`.
- **`types.ts`** — TypeScript mirrors of the API's response shapes (`User`, `Org`, `Member`, `Invitation`, `Deployment`, `SessionRow`, `Summary`).
- **`use-telemetry-socket.ts`** — opens a WebSocket to the API's `/ws/organizations/{id}` (Redis-backed pub/sub) and calls a refetch whenever new telemetry is ingested, so the org dashboard live-updates (with a "● Live" badge). `http→ws` derived from `NEXT_PUBLIC_API_URL`.

## Page composition

Pages are client components. Protected pages call `useRequireAuth()` (gate) + `useAuth()` (token), fetch with `api()` in `useEffect`, and render glass cards. The org detail page composes sub-cards (Sessions, Members, Invite, Deployments) and the dollars-prevented hero.

## How it fits the system

The web app is a pure client of the **Turnstile API** — all auth, org, and telemetry data comes over REST with a JWT. It never talks to the Go data plane or Postgres directly. It runs via `next dev` in development and `next start` (or a static export) in production.
