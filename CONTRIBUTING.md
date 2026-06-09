# Contributing to Turnstile

Thanks for your interest in contributing. Turnstile is a monorepo with three
independent services, each with its own toolchain and test suite:

| Path                       | Stack                | What it is                          |
| -------------------------- | -------------------- | ----------------------------------- |
| `go/`                      | Go 1.22+ (stdlib)    | Data-plane proxy + circuit breaker  |
| `python/turnstile-api/`    | Python 3.13, FastAPI | Control-plane API + database owner  |
| `javascript/turnstile-web/`| Next.js 16, React 19 | Dashboard web app                   |

## Development setup

See [`docs/RUNNING_LOCALLY.md`](docs/RUNNING_LOCALLY.md) for the full end-to-end
walkthrough. Each subproject also has its own `README.md` with local commands.

Quick reference (each uses [`just`](https://github.com/casey/just)):

```sh
# Go data plane
cd go && just test          # gofmt, vet, test (incl. the latency gate)

# Python API (needs local Postgres + Redis)
cd python/turnstile-api && just install && just db-create-all && just db-migrate-all && just test

# Web dashboard (API must be running on :8000)
cd javascript/turnstile-web && npm ci && npm run build && npx playwright test
```

## Coding conventions

These are enforced in review and, where possible, in CI:

- **Tests are not optional.** Every change ships with tests. We practice
  test-driven development: write the failing test, then make it pass. CI must be
  green before merge.
- **Code should be self-documenting.** Prefer clear names and small functions
  over comments. A comment that restates *what* the code does is a smell —
  delete it. Keep comments only for the non-obvious *why* (a tricky invariant, a
  workaround, a reference to a spec).
- **Match the surrounding style.** Run the formatter for the stack you touch:
  `gofmt` (Go), `ruff format` (Python), `eslint`/Prettier defaults (TypeScript).
- **No secrets in the repo.** Configuration is via environment variables; commit
  only `.env.example` files with placeholder values.
- **Keep the dependency surface small.** The Go data plane is stdlib-only by
  design — adding a dependency there requires a strong justification.

## Pull requests

1. Branch from `main`.
2. Keep PRs focused; one concern per PR.
3. Ensure the relevant CI workflow(s) pass (`go`, `api`, `web`).
4. Describe *why* the change is needed, not just what changed.

By contributing, you agree that your contributions are licensed under the
[MIT License](LICENSE).
