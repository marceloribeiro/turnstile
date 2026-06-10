# turnstile-marketing

The public marketing site for **Turnstile** — the transparent proxy that kills
the one rogue LLM session without taking down the rest of your app.

Single-page Next.js landing site, built to share the dashboard's design system
(`turnstile-web`): calm graphite, one steel-blue accent, semantic green for
*dollars prevented*. Target domain: **turnstileguard.com**.

| | |
| --- | --- |
| Stack | Next.js 16 · React 19 · Tailwind v4 |
| Output | `standalone` (self-hosted via rsync + systemd) |
| Sections | Hero · Problem · Dollars prevented · How it works · Features · Base-URL swap · Architecture · CTA |

## Develop

```bash
npm install
npm run dev          # http://localhost:3000  (or PORT=3100 npm run dev)
npx tsc --noEmit     # typecheck
```

## Notes

- Content is sourced from the repo root `README.md` and `docs/` so the messaging
  stays in sync with the product.
- Screenshots in `public/` (`dashboard.png`, `org-overview.png`,
  `architecture.svg`) are copied from `docs/images/`.
- Entrance animations are deliberately **not** opacity-gated — every section is
  visible without JS so crawlers and static captures render the full page.
