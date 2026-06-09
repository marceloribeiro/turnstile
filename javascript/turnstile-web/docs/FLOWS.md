# Flows — Turnstile Web

## 1. App bootstrap & auth state

```
RootLayout → <AuthProvider>
   on mount: token = localStorage["turnstile_token"]
             if token → api("/me", {token}) → setUser   (else clear)
   loading=true until resolved
/ (page.tsx): when !loading → redirect to /dashboard (user) or /login
```

## 2. Login / register

```
/login: form → useAuth().login(email, password)
   → api POST /login → {user, token} → persist to localStorage + context
   → router.push("/dashboard")
/register: same shape via /register; on the server side this also auto-accepts
   any pending invitations for the email (platform rule 1).
Errors: ApiError.message rendered in <ErrorText>.
```

## 3. Protected pages

```
page: const {user, loading} = useRequireAuth()   // redirects to /login if unauthed
      const {token} = useAuth()
      useEffect → api(path, {token}) → setState → render glass cards
```

## 4. Dashboard (organizations)

```
/dashboard:
   GET /organizations → grid of GlassCards (each links to /organizations/[id])
   create form → POST /organizations → refresh
```

## 5. Organization area (tabbed)

`app/organizations/[id]/layout.tsx` provides the chrome — org name + a tab nav (Overview / Members /
Developer) — and guards auth. Each tab is its own page (its own data fetch); the nav active state comes
from `usePathname()`.

```
layout.tsx               GET /organizations/{id} → name; tab nav (active via usePathname)

/organizations/[id]              (Overview)
   GET /summary  → HERO: dollars-prevented (accent) + sessions/requests/cost/blocks + Live badge
   GET /sessions → sessions table (session_key, source, model, reqs, cost, blocks)
   useTelemetrySocket → live refetch on ingest

/organizations/[id]/members      (Members)
   GET /members      → members list (role chips)
   GET /invitations  → pending invitations
   InviteCard: POST /invitations {email, role} → emailed via API → reloads

/organizations/[id]/developers   (Developer)
   GET /deployments  → list (name, key prefix, last seen)
   POST /deployments {name} → ingest_key shown ONCE in a highlighted box + a copy-paste .env snippet
```

## 6. Invitation accept

```
/invitations/accept?token=…  (useSearchParams, Suspense-wrapped):
   if not signed in → prompt to /login
   else → button → POST /invitations/accept {token}
        → success → redirect /dashboard
   (already-registered invitees use this; brand-new signups are auto-accepted at /register)
```

## 7. Live updates (WebSocket)

```
org page mounts → useTelemetrySocket(orgId, token, load)
   opens WebSocket ws://API/ws/organizations/{id}?token=<jwt>
   on each message (telemetry ingested) → calls load() → refetch /summary + /sessions
   connected state → "● Live" badge; disconnect → "○ Offline"
```

The API broadcasts over Redis pub/sub whenever the Go container's telemetry lands, so the
dashboard updates on its own — no polling, no manual refresh. (Freshness is still bounded by how
often the Go container pushes, `TURNSTILE_INGEST_INTERVAL`.)

## Selector contract (for Playwright)

Pages use accessible, stable handles so specs read like user stories: form fields via `getByPlaceholder`, actions via `getByRole("button", {name})`, headings via `getByRole("heading")`. See `tests/specs/auth.spec.ts`.
