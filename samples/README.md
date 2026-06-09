# Samples — talk to an LLM through Turnstile

Three tiny interactive CLIs, one per provider. Each uses the provider's **official
SDK with only the base URL swapped to Turnstile** — the whole point of Turnstile:
no SDK change, no code change, just a different base URL. You type a question and
the answer streams back, having passed through the Turnstile data plane (which
meters it, enforces budgets/loops, and forwards to the provider).

| Sample | SDK | Turnstile base URL | Routes to |
| --- | --- | --- | --- |
| [`openrouter-sample`](openrouter-sample) | `openai` (OpenRouter is OpenAI-compatible) | `http://localhost:8080/api/v1` | OpenRouter (fallback adapter) |
| [`openai-sample`](openai-sample) | `openai` | `http://localhost:8080/v1` | OpenAI adapter |
| [`anthropic-sample`](anthropic-sample) | `anthropic` | `http://localhost:8080` | Anthropic adapter |

The base URL is the only thing that differs from a normal integration. Turnstile
routes by request shape: `/api/v1/*` → OpenRouter, `/v1/chat/completions` &
`/v1/responses` → OpenAI, `/v1/messages` → Anthropic.

## Prerequisites

1. **Turnstile data plane running on `:8080`.** From the repo root:
   ```sh
   cd go && just up        # data plane :8080, control plane :8081
   ```
   Tip: run it with `TURNSTILE_DEBUG=true just up` to see one log line per request
   (adapter chosen, session, tokens parsed) as your questions go through.
   > The OpenAI and Anthropic adapters must be present in the running binary.
   > The OpenRouter, OpenAI, and Anthropic adapters are all on `main`.
2. **A provider API key** for whichever sample you run (your own key — Turnstile
   passes it straight through and never stores it).
3. **Python 3.9+** and [`just`](https://github.com/casey/just).

## Run one

```sh
cd samples/openai-sample          # or openrouter-sample / anthropic-sample
cp .env.example .env              # then put your API key in .env
just install                      # one-time: venv + the provider SDK
just console                      # type a question; answer streams back
```

Example session:

```
Connected via Turnstile → http://localhost:8080/v1  (model: gpt-4o-mini)
Type a question. Blank line or Ctrl-C to quit.

you> what is a turnstile, in one sentence?
assistant> A turnstile is a gate that lets one person through at a time…

you>
```

Each sample sends an `X-Turnstile-Session` header so its conversation groups into
a single session. It's metered **in-memory** the moment it runs — no setup, no
ingest key — and visible on the control plane:

```sh
curl http://localhost:8081/v1/sessions   # your session, tokens, cost
```

### Seeing it in the web dashboard

That's a separate, optional step. The samples do **not** use a Turnstile ingest
key — the key lives in the **data plane's** config (`go/.env`) and is what ships
your session to the platform so it appears in the dashboard:

1. In the web app: sign in → open an organization → create a project → create a
   deployment → copy its ingest key (`ts_...`).
2. In `go/.env` set `TURNSTILE_INGEST_URL=http://localhost:8000/ingest/sessions`
   and `TURNSTILE_INGEST_KEY=ts_...`, then restart the data plane (`just up`).
3. View **that** organization — your session appears within one ingest interval.

Without this, everything still works and meters; it just isn't shipped off-box
(it stays on `:8081`). Each sample's `.env.example` repeats these steps.
