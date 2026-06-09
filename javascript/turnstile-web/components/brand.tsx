export function Brand({ size = 32, withText = false }: { size?: number; withText?: boolean }) {
  return (
    <span className="inline-flex items-center gap-2">
      <svg width={size} height={size} viewBox="0 0 48 48" fill="none" aria-hidden>
        {/* Turnstile rotor in its housing (top view): a ring with four arms and
            a hub — controlled passage, one at a time. Flat single accent. */}
        <circle cx="24" cy="24" r="19" stroke="var(--accent)" strokeWidth="2.4" />
        <g stroke="var(--accent)" strokeWidth="2.6" strokeLinecap="round">
          <line x1="24" y1="24" x2="24" y2="11" />
          <line x1="24" y1="24" x2="24" y2="37" />
          <line x1="24" y1="24" x2="11" y2="24" />
          <line x1="24" y1="24" x2="37" y2="24" />
        </g>
        <circle cx="24" cy="24" r="3.6" fill="var(--accent)" />
      </svg>
      {withText && <span className="text-lg font-bold tracking-tight">Turnstile</span>}
    </span>
  );
}
