/* Horizontal lockup — light variant (off-white wordmark + steel-blue mark) so it
   reads on the dark topbar and footer. Source art lives in public/. */
export function Logo({ className = "" }: { className?: string }) {
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src="/turnstile-logo-light.png"
      alt="Turnstile"
      width={1207}
      height={413}
      className={`w-auto select-none ${className || "h-7"}`}
    />
  );
}
