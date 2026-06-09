import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from "react";

export function GlassCard({
  children,
  className = "",
  strong = false,
}: {
  children: ReactNode;
  className?: string;
  strong?: boolean;
}) {
  return (
    <div className={`${strong ? "glass-strong" : "glass"} rounded-3xl ${className}`}>
      {children}
    </div>
  );
}

export function Button({
  children,
  variant = "primary",
  className = "",
  loading = false,
  disabled,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost";
  loading?: boolean;
}) {
  return (
    <button
      className={`btn ${variant === "primary" ? "btn-primary" : "btn-ghost"} ${className}`}
      disabled={loading || disabled}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading && <Spinner />}
      {children}
    </button>
  );
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input className="glass-input" {...props} />;
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="label">{label}</span>
      {children}
    </label>
  );
}

export function Spinner() {
  return (
    <span
      aria-hidden
      className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white"
    />
  );
}

export function CenterSpinner() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center">
      <span className="inline-block h-8 w-8 animate-spin rounded-full border-2 border-white/20 border-t-white/80" />
    </div>
  );
}

export function LiveBadge({ live }: { live: boolean }) {
  return (
    <span className="chip" title={live ? "Receiving live updates" : "Not connected"}>
      <span
        className={`inline-block h-2 w-2 rounded-full ${
          live ? "animate-pulse bg-emerald-400" : "bg-white/30"
        }`}
      />
      {live ? "Live" : "Offline"}
    </span>
  );
}

export function SectionHeader({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div className="mb-5">
      <h2 className="text-xl font-semibold tracking-tight">{title}</h2>
      {subtitle && <p className="muted mt-1 text-sm">{subtitle}</p>}
    </div>
  );
}

export function Stat({
  label,
  value,
  accent = false,
}: {
  label: string;
  value: ReactNode;
  accent?: boolean;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-semibold uppercase tracking-wide text-white/50">{label}</span>
      <span
        className={`tnum font-semibold ${accent ? "stat-accent" : "text-white"}`}
        style={{ fontSize: accent ? "2.6rem" : "1.5rem", lineHeight: 1.1 }}
      >
        {value}
      </span>
    </div>
  );
}

export function ErrorText({ children }: { children: ReactNode }) {
  if (!children) return null;
  return (
    <p className="rounded-xl border border-rose-400/30 bg-rose-500/15 px-3 py-2 text-sm text-rose-200">
      {children}
    </p>
  );
}

export function usd(n: number): string {
  if (n === 0) return "$0";
  if (n < 0.01) return "$" + n.toFixed(6);
  return "$" + n.toFixed(2);
}
