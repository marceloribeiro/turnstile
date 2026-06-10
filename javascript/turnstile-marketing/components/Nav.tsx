import { Logo } from "./Logo";

const GITHUB = "https://github.com/marceloribeiro/turnstile";

export function Nav() {
  return (
    <header className="topbar sticky top-0 z-50">
      <nav className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
        <a href="#top" className="shrink-0">
          <Logo />
        </a>
        <div className="hidden items-center gap-8 text-sm md:flex">
          <a href="#problem" className="muted transition-colors hover:text-[var(--fg)]">
            Why
          </a>
          <a href="#how" className="muted transition-colors hover:text-[var(--fg)]">
            How it works
          </a>
          <a href="#features" className="muted transition-colors hover:text-[var(--fg)]">
            Features
          </a>
          <a
            href="#architecture"
            className="muted transition-colors hover:text-[var(--fg)]"
          >
            Architecture
          </a>
        </div>
        <div className="flex items-center gap-2.5">
          <a
            href={GITHUB}
            target="_blank"
            rel="noopener noreferrer"
            className="btn btn-ghost hidden sm:inline-flex"
          >
            <svg width="17" height="17" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
              <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0016 8c0-4.42-3.58-8-8-8z" />
            </svg>
            GitHub
          </a>
          <a href="#cta" className="btn btn-primary">
            Get started
          </a>
        </div>
      </nav>
    </header>
  );
}
