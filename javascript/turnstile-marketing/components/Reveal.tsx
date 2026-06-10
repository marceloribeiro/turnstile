/**
 * One-shot entrance wrapper. The animation is pure CSS (`.reveal`) and always
 * resolves to the visible end state, so content is never hidden from crawlers,
 * no-JS visitors, or full-page screenshots. `delay` staggers the entrance.
 */
export function Reveal({
  children,
  className = "",
  delay = 0,
  as: Tag = "div",
}: {
  children: React.ReactNode;
  className?: string;
  delay?: number;
  as?: keyof React.JSX.IntrinsicElements;
}) {
  const Component = Tag as React.ElementType;
  return (
    <Component
      className={`reveal ${className}`}
      style={delay ? { animationDelay: `${delay}ms` } : undefined}
    >
      {children}
    </Component>
  );
}
