export function Logo({ className = "" }: { className?: string }) {
  return (
    <span className={`inline-flex items-center gap-2 ${className}`}>
      <svg
        width="22"
        height="22"
        viewBox="0 0 32 32"
        fill="none"
        aria-hidden="true"
      >
        <circle cx="16" cy="16" r="9" stroke="#7fb0dd" strokeWidth="2.4" />
        <path
          d="M16 11.5v9M11.5 16h9"
          stroke="#7fb0dd"
          strokeWidth="2.4"
          strokeLinecap="round"
        />
      </svg>
      <span className="text-[1.05rem] font-semibold tracking-tight">
        Turnstile
      </span>
    </span>
  );
}
