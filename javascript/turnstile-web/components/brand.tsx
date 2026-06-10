/* Horizontal lockup — light variant (off-white wordmark + steel-blue mark) so it
   reads on the dark topbar and auth screens. Same art as the marketing site. The
   `withText` prop is kept for call-site compatibility; the lockup always includes
   the wordmark. */
export function Brand({ size = 32 }: { size?: number; withText?: boolean }) {
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src="/turnstile-logo-light.png"
      alt="Turnstile"
      width={1207}
      height={413}
      style={{ height: size, width: "auto" }}
      className="select-none"
    />
  );
}
