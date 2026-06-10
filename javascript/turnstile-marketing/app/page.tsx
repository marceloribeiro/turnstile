import { Logo } from "@/components/Logo";
import { Nav } from "@/components/Nav";
import { Reveal } from "@/components/Reveal";

const GITHUB = "https://github.com/marceloribeiro/turnstile";

export default function Home() {
  return (
    <div id="top">
      <Nav />
      <main>
        <Hero />
        <Providers />
        <Problem />
        <DollarsPrevented />
        <HowItWorks />
        <Features />
        <CodeSwap />
        <Architecture />
        <CtaBand />
      </main>
      <Footer />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Hero                                                                */
/* ------------------------------------------------------------------ */
function Hero() {
  return (
    <section className="relative mx-auto max-w-6xl px-5 pt-20 pb-16 md:pt-28 md:pb-24">
      <div className="grid items-center gap-14 lg:grid-cols-[1.05fr_0.95fr]">
        <div>
          <Reveal>
            <span className="chip">
              <span className="relative inline-flex h-1.5 w-1.5">
                <span className="pulse inline-flex h-1.5 w-1.5 rounded-full" />
              </span>
              Real-time circuit breaker for AI sessions
            </span>
          </Reveal>
          <Reveal delay={60}>
            <h1 className="mt-6 text-4xl font-semibold leading-[1.08] tracking-tight md:text-6xl">
              Kill the one rogue
              <br />
              LLM session.
              <br />
              <span className="muted">Not your whole app.</span>
            </h1>
          </Reveal>
          <Reveal delay={120}>
            <p className="mt-6 max-w-xl text-lg leading-relaxed muted">
              Turnstile is a transparent proxy that meters every AI session in
              real time and trips a circuit breaker the instant{" "}
              <span className="text-[var(--fg)]">one</span> session goes rogue —
              a runaway agent loop or a blown budget — without taking down the
              rest of your app.
            </p>
          </Reveal>
          <Reveal delay={180}>
            <div className="mt-8 flex flex-wrap items-center gap-3">
              <a href="#how" className="btn btn-primary">
                See how it works
              </a>
              <a
                href={GITHUB}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-ghost"
              >
                View on GitHub
              </a>
            </div>
          </Reveal>
          <Reveal delay={240}>
            <p className="mt-6 text-sm faint">
              Drop-in. Swap one base URL — no SDK, no code changes. Your provider
              key passes through and is never stored.
            </p>
          </Reveal>
        </div>

        {/* Hero instrument card — the dollars-prevented hero metric */}
        <Reveal delay={140} className="lg:justify-self-end">
          <div className="glass-strong w-full max-w-md rounded-2xl p-6">
            <div className="flex items-center justify-between">
              <span className="chip">Live overview</span>
              <span className="flex items-center gap-1.5 text-xs muted">
                <span className="relative inline-flex h-1.5 w-1.5">
                  <span className="pulse inline-flex h-1.5 w-1.5 rounded-full" />
                </span>
                Live
              </span>
            </div>
            <div className="mt-6">
              <div className="text-[0.72rem] font-semibold uppercase tracking-wider faint">
                Dollars prevented
              </div>
              <div className="tnum mt-1 text-5xl font-semibold stat-accent">
                $13.60
              </div>
              <div className="mt-1 text-sm muted">
                Saved by killing runaway loops and budget overruns.
              </div>
            </div>
            <div className="mt-6 space-y-2.5">
              <SessionRow
                name="sess:research-loop-3"
                model="claude-3-5-sonnet"
                cost="$6.75"
                blocks={5}
              />
              <SessionRow
                name="sess:checkout-agent-1"
                model="gpt-4o"
                cost="$12.40"
                blocks={0}
              />
              <SessionRow
                name="sess:batch-summarizer"
                model="gpt-4o-mini"
                cost="$3.20"
                blocks={1}
              />
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function SessionRow({
  name,
  model,
  cost,
  blocks,
}: {
  name: string;
  model: string;
  cost: string;
  blocks: number;
}) {
  return (
    <div className="flex items-center justify-between rounded-lg border border-white/5 bg-white/[0.02] px-3 py-2.5">
      <div className="min-w-0">
        <div className="truncate font-mono text-xs">{name}</div>
        <div className="truncate text-[0.7rem] faint">{model}</div>
      </div>
      <div className="flex items-center gap-3 pl-3">
        <span className="tnum text-sm">{cost}</span>
        {blocks > 0 ? (
          <span
            className="tnum rounded-md px-1.5 py-0.5 text-[0.7rem] font-semibold"
            style={{
              color: "var(--danger)",
              background: "rgba(233,140,140,0.12)",
              border: "1px solid rgba(233,140,140,0.25)",
            }}
          >
            {blocks} blocked
          </span>
        ) : (
          <span className="tnum text-[0.7rem] faint">0 blocked</span>
        )}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Provider strip                                                      */
/* ------------------------------------------------------------------ */
function Providers() {
  const providers = ["OpenAI", "Anthropic", "Google Gemini", "OpenRouter"];
  return (
    <section className="mx-auto max-w-6xl px-5 pb-8">
      <Reveal className="hairline pt-10">
        <p className="text-center text-xs font-semibold uppercase tracking-[0.14em] faint">
          Works with any OpenAI-compatible endpoint — direct provider adapters too
        </p>
        <div className="mt-6 flex flex-wrap items-center justify-center gap-x-10 gap-y-4">
          {providers.map((p) => (
            <span
              key={p}
              className="text-lg font-semibold tracking-tight muted transition-colors hover:text-[var(--fg)]"
            >
              {p}
            </span>
          ))}
        </div>
      </Reveal>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/* The problem                                                         */
/* ------------------------------------------------------------------ */
function Problem() {
  return (
    <section id="problem" className="mx-auto max-w-6xl px-5 py-20 md:py-28">
      <Reveal>
        <span className="kicker">The problem</span>
        <h2 className="mt-3 max-w-3xl text-3xl font-semibold tracking-tight md:text-4xl">
          Teams running AI agents in production face two problems with no good
          answer.
        </h2>
      </Reveal>
      <div className="mt-12 grid gap-5 md:grid-cols-2">
        <Reveal>
          <div className="glass h-full rounded-2xl p-7">
            <div
              className="flex h-10 w-10 items-center justify-center rounded-lg"
              style={{
                color: "var(--danger)",
                background: "rgba(233,140,140,0.1)",
                border: "1px solid rgba(233,140,140,0.22)",
              }}
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <path
                  d="M3 17l5-6 4 4 5-7 4 5"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>
            <h3 className="mt-5 text-xl font-semibold">
              Costs spike with no warning
            </h3>
            <p className="mt-3 leading-relaxed muted">
              A prompt-injection loop, a retry storm, or a single greedy agent
              can burn through your month&apos;s budget in an afternoon. By the
              time the invoice lands, the money is gone.
            </p>
          </div>
        </Reveal>
        <Reveal delay={80}>
          <div className="glass h-full rounded-2xl p-7">
            <div
              className="flex h-10 w-10 items-center justify-center rounded-lg"
              style={{
                color: "var(--danger)",
                background: "rgba(233,140,140,0.1)",
                border: "1px solid rgba(233,140,140,0.22)",
              }}
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <path
                  d="M18.36 5.64a9 9 0 11-12.72 0M12 2v8"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>
            <h3 className="mt-5 text-xl font-semibold">
              No way to stop just one
            </h3>
            <p className="mt-3 leading-relaxed muted">
              Your only kill switch is the global API key. Revoke it and every
              session dies — every user, every agent, the whole app. There&apos;s
              no scalpel, only the plug.
            </p>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/* Dollars prevented                                                   */
/* ------------------------------------------------------------------ */
function DollarsPrevented() {
  return (
    <section className="mx-auto max-w-6xl px-5 py-20 md:py-28">
      <div className="grid items-center gap-14 lg:grid-cols-[0.9fr_1.1fr]">
        <Reveal>
          <div>
            <span className="kicker">One number that matters</span>
            <h2 className="mt-3 text-3xl font-semibold tracking-tight md:text-4xl">
              Dollars prevented, not marketing math.
            </h2>
            <p className="mt-5 leading-relaxed muted">
              Turnstile surfaces a single hero metric: the estimated spend it
              blocked when a session crossed its budget ceiling or tripped loop
              detection — priced from that session&apos;s own recent
              cost-per-call. So the number reflects{" "}
              <span className="text-[var(--fg)]">real averted spend</span>, not a
              made-up multiplier.
            </p>
            <ul className="mt-7 space-y-3.5">
              {[
                "Pre-execution enforcement — it stops the call before it runs, not after the bill arrives.",
                "Priced from the session's actual recent cost-per-call.",
                "Rolled up per org, per model, and across your whole account.",
              ].map((t) => (
                <li key={t} className="flex gap-3">
                  <Check />
                  <span className="muted">{t}</span>
                </li>
              ))}
            </ul>
          </div>
        </Reveal>
        <Reveal delay={100}>
          <figure className="glass-strong overflow-hidden rounded-2xl p-2">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src="/org-overview.png"
              alt="Turnstile organization overview — a live dollars-prevented hero with a per-session breakdown of model, cost, and blocks."
              className="w-full rounded-xl"
              width={1280}
              height={920}
            />
          </figure>
        </Reveal>
      </div>
    </section>
  );
}

function Check() {
  return (
    <span
      className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full"
      style={{
        color: "var(--positive)",
        background: "rgba(111,207,158,0.12)",
        border: "1px solid rgba(111,207,158,0.28)",
      }}
    >
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
        <path
          d="M5 13l4 4L19 7"
          stroke="currentColor"
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </span>
  );
}

/* ------------------------------------------------------------------ */
/* How it works                                                        */
/* ------------------------------------------------------------------ */
function HowItWorks() {
  const steps = [
    {
      n: "01",
      t: "Swap one base URL",
      d: "Point your LLM client at Turnstile instead of the provider. No SDK, no code changes. Your provider key rides along and is never stored.",
    },
    {
      n: "02",
      t: "Meter every session",
      d: "The Go data plane resolves each request to a session and meters tokens and cost in real time — everything off the hot path.",
    },
    {
      n: "03",
      t: "Enforce before forwarding",
      d: "A synchronous gate checks the budget ceiling, loop detection, and manual kill — then forwards to the provider and streams back byte-for-byte.",
    },
    {
      n: "04",
      t: "Watch it live",
      d: "Hashed aggregates (never raw prompts or keys) post to the control plane and stream to your dashboard over WebSocket — sessions, spend, dollars prevented.",
    },
  ];
  return (
    <section id="how" className="mx-auto max-w-6xl px-5 py-20 md:py-28">
      <Reveal>
        <span className="kicker">How it works</span>
        <h2 className="mt-3 max-w-2xl text-3xl font-semibold tracking-tight md:text-4xl">
          Point your base URL at Turnstile. The rest is automatic.
        </h2>
      </Reveal>
      <div className="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {steps.map((s, i) => (
          <Reveal key={s.n} delay={i * 70}>
            <div className="glass h-full rounded-2xl p-6">
              <div className="tnum text-2xl font-semibold text-[var(--accent)]">
                {s.n}
              </div>
              <h3 className="mt-4 text-lg font-semibold">{s.t}</h3>
              <p className="mt-2.5 text-sm leading-relaxed muted">{s.d}</p>
            </div>
          </Reveal>
        ))}
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/* Features                                                            */
/* ------------------------------------------------------------------ */
function Features() {
  const features = [
    {
      t: "Drop-in",
      d: "Swap one base URL. Keep your provider API key — it's passed through, never stored.",
      icon: (
        <path
          d="M13 2L3 14h7l-1 8 10-12h-7l1-8z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinejoin="round"
        />
      ),
    },
    {
      t: "Session-precise",
      d: "Kills the one rogue trajectory, not the global API key. Every other session keeps flowing.",
      icon: (
        <>
          <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="2" />
          <circle cx="12" cy="12" r="3.5" stroke="currentColor" strokeWidth="2" />
        </>
      ),
    },
    {
      t: "Real-time",
      d: "Pre-execution enforcement — budget ceilings, loop detection, manual kill — not a retrospective dashboard.",
      icon: (
        <path
          d="M12 7v5l3 2M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      ),
    },
    {
      t: "Negligible overhead",
      d: "Microsecond-scale added latency — measured, self-instrumented, with a test that fails if it regresses.",
      icon: (
        <path
          d="M12 14l4-4M21 12a9 9 0 10-18 0M12 14a2 2 0 100-4 2 2 0 000 4z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      ),
    },
    {
      t: "Fail-open",
      d: "A fault in Turnstile forwards the request rather than breaking your app — configurable to fail-closed.",
      icon: (
        <path
          d="M7 11V7a5 5 0 019.9-1M5 11h14v10H5V11z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      ),
    },
    {
      t: "Private",
      d: "Raw prompts and API keys are never persisted — only salted hashes and numeric aggregates leave the data plane.",
      icon: (
        <path
          d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinejoin="round"
        />
      ),
    },
  ];
  return (
    <section id="features" className="mx-auto max-w-6xl px-5 py-20 md:py-28">
      <Reveal>
        <span className="kicker">What you get</span>
        <h2 className="mt-3 max-w-2xl text-3xl font-semibold tracking-tight md:text-4xl">
          Built for the hot path of production AI.
        </h2>
      </Reveal>
      <div className="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {features.map((f, i) => (
          <Reveal key={f.t} delay={(i % 3) * 70}>
            <div className="glass h-full rounded-2xl p-7">
              <div className="flex h-11 w-11 items-center justify-center rounded-lg border border-white/10 bg-white/[0.03] text-[var(--accent)]">
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                  {f.icon}
                </svg>
              </div>
              <h3 className="mt-5 text-lg font-semibold">{f.t}</h3>
              <p className="mt-2.5 text-sm leading-relaxed muted">{f.d}</p>
            </div>
          </Reveal>
        ))}
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/* Code swap                                                           */
/* ------------------------------------------------------------------ */
function CodeSwap() {
  return (
    <section className="mx-auto max-w-6xl px-5 py-20 md:py-28">
      <div className="grid items-center gap-14 lg:grid-cols-2">
        <Reveal>
          <div>
            <span className="kicker">One line</span>
            <h2 className="mt-3 text-3xl font-semibold tracking-tight md:text-4xl">
              The only change is the base URL.
            </h2>
            <p className="mt-5 leading-relaxed muted">
              Keep your SDK, your model names, your API key. Turnstile speaks the
              same OpenAI-compatible protocol and forwards everything
              byte-for-byte — it just meters and enforces on the way through.
            </p>
            <div className="mt-7 flex flex-wrap gap-2">
              {["No SDK", "No code rewrite", "Key passed through", "Streaming preserved"].map(
                (c) => (
                  <span key={c} className="chip">
                    {c}
                  </span>
                )
              )}
            </div>
          </div>
        </Reveal>
        <Reveal delay={100}>
          <div className="code p-5">
            <pre className="whitespace-pre">
              <code>
                <span className="c-dim">{`# before — straight to the provider`}</span>
                {"\n"}
                client = OpenAI(
                {"\n"}
                {"    "}base_url=
                <span className="c-danger">{`"https://api.openai.com/v1"`}</span>,
                {"\n"}
                {"    "}api_key=os.environ[<span className="c-accent">{`"OPENAI_API_KEY"`}</span>],
                {"\n"}
                )
                {"\n\n"}
                <span className="c-dim">{`# after — through Turnstile (that's it)`}</span>
                {"\n"}
                client = OpenAI(
                {"\n"}
                {"    "}base_url=
                <span className="c-pos">{`"https://turnstileguard.com/v1"`}</span>,
                {"\n"}
                {"    "}api_key=os.environ[<span className="c-accent">{`"OPENAI_API_KEY"`}</span>],{" "}
                <span className="c-dim">{`# passed through`}</span>
                {"\n"}
                )
              </code>
            </pre>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/* Architecture                                                        */
/* ------------------------------------------------------------------ */
function Architecture() {
  const planes = [
    {
      t: "Go data plane",
      tag: "Hot path",
      d: "The transparent proxy and circuit breaker. Meters, enforces, and forwards — Go 1.22, stdlib only.",
    },
    {
      t: "FastAPI control plane",
      tag: "Source of truth",
      d: "Multi-tenant API and the sole database owner. Upserts hashed aggregates into Postgres, publishes over Redis.",
    },
    {
      t: "Next.js dashboard",
      tag: "Account UI",
      d: "Live telemetry over REST + WebSocket. Sessions, spend, and dollars prevented, updating in real time.",
    },
  ];
  return (
    <section id="architecture" className="mx-auto max-w-6xl px-5 py-20 md:py-28">
      <Reveal>
        <span className="kicker">Architecture</span>
        <h2 className="mt-3 max-w-2xl text-3xl font-semibold tracking-tight md:text-4xl">
          Three planes. The hot path stays thin.
        </h2>
        <p className="mt-5 max-w-2xl leading-relaxed muted">
          Your app points its LLM base URL at the Go data plane, which meters and
          enforces in real time and forwards to the provider. It posts hashed
          aggregates — never raw prompts or keys — to the FastAPI control plane,
          which owns Postgres and pushes live updates over Redis.
        </p>
      </Reveal>
      <Reveal delay={100}>
        <figure className="glass mt-12 overflow-hidden rounded-2xl p-6 md:p-10">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/architecture.svg"
            alt="Turnstile architecture: your app → Go data plane (meter + enforce + forward) → provider; hashed aggregates → FastAPI control plane → Postgres + Redis → Next.js dashboard."
            className="mx-auto w-full max-w-4xl"
          />
        </figure>
      </Reveal>
      <div className="mt-5 grid gap-5 md:grid-cols-3">
        {planes.map((p, i) => (
          <Reveal key={p.t} delay={i * 70}>
            <div className="glass h-full rounded-2xl p-6">
              <div className="flex items-center justify-between">
                <h3 className="text-lg font-semibold">{p.t}</h3>
                <span className="chip">{p.tag}</span>
              </div>
              <p className="mt-3 text-sm leading-relaxed muted">{p.d}</p>
            </div>
          </Reveal>
        ))}
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/* CTA band                                                            */
/* ------------------------------------------------------------------ */
function CtaBand() {
  return (
    <section id="cta" className="mx-auto max-w-6xl px-5 py-20 md:py-28">
      <Reveal>
        <div className="glass-strong relative overflow-hidden rounded-3xl px-8 py-16 text-center md:px-16 md:py-20">
          <div
            className="pointer-events-none absolute inset-x-0 -top-1/2 h-full"
            style={{
              background:
                "radial-gradient(60% 60% at 50% 50%, rgba(127,176,221,0.16), transparent 70%)",
            }}
          />
          <h2 className="relative mx-auto max-w-2xl text-3xl font-semibold tracking-tight md:text-5xl">
            Point one base URL.
            <br />
            Watch dollars prevented climb.
          </h2>
          <p className="relative mx-auto mt-5 max-w-xl leading-relaxed muted">
            Open-source, MIT-licensed, and running end-to-end today. Stand it up
            locally in minutes — your app, through Turnstile, to any provider.
          </p>
          <div className="relative mt-9 flex flex-wrap items-center justify-center gap-3">
            <a
              href={GITHUB}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-primary"
            >
              Get started on GitHub
            </a>
            <a
              href={`${GITHUB}/blob/main/docs/RUNNING_LOCALLY.md`}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-ghost"
            >
              Read the quick start
            </a>
          </div>
        </div>
      </Reveal>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/* Footer                                                              */
/* ------------------------------------------------------------------ */
function Footer() {
  return (
    <footer className="hairline">
      <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-6 px-5 py-12 md:flex-row">
        <div>
          <Logo />
          <p className="mt-3 max-w-xs text-sm faint">
            The session layer for the autonomous AI era — where every agent&apos;s
            spend and behavior is observed and governed.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-x-8 gap-y-3 text-sm">
          <a href="#how" className="muted transition-colors hover:text-[var(--fg)]">
            How it works
          </a>
          <a
            href="#features"
            className="muted transition-colors hover:text-[var(--fg)]"
          >
            Features
          </a>
          <a
            href="#architecture"
            className="muted transition-colors hover:text-[var(--fg)]"
          >
            Architecture
          </a>
          <a
            href={GITHUB}
            target="_blank"
            rel="noopener noreferrer"
            className="muted transition-colors hover:text-[var(--fg)]"
          >
            GitHub
          </a>
        </div>
      </div>
      <div className="hairline">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-2 px-5 py-6 text-xs faint md:flex-row">
          <span>© 2026 Turnstile · MIT Licensed</span>
          <span>Raw prompts and API keys are never persisted.</span>
        </div>
      </div>
    </footer>
  );
}
