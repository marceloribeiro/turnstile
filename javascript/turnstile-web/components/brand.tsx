export function Brand({ size = 32, withText = false }: { size?: number; withText?: boolean }) {
  return (
    <span className="inline-flex items-center gap-2">
      <svg width={size} height={size} viewBox="0 0 48 48" fill="none" aria-hidden>
        <defs>
          <linearGradient id="turnstile-shield" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#6366f1" />
            <stop offset="0.5" stopColor="#8b5cf6" />
            <stop offset="1" stopColor="#d946ef" />
          </linearGradient>
        </defs>
        <path
          d="M24 3l16 6v11c0 10-6.8 18.8-16 22-9.2-3.2-16-12-16-22V9l16-6z"
          fill="url(#turnstile-shield)"
        />
        <path
          d="M16 24.5l5.5 5.5L33 18"
          stroke="white"
          strokeWidth="3.4"
          strokeLinecap="round"
          strokeLinejoin="round"
          fill="none"
        />
      </svg>
      {withText && <span className="text-lg font-bold tracking-tight">Turnstile</span>}
    </span>
  );
}
