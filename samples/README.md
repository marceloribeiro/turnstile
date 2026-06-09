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
   > OpenAI is on `main`; Anthropic lands when its PR merges (until then, build the
   > data plane from the branch that has it).
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

Each sample sends an `X-Turnstile-Session` header so its conversation shows up as
a single session in the control plane / dashboard. Watch it live:

```sh
curl http://localhost:8081/v1/sessions   # or open the web dashboard
```
