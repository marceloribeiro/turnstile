"use client";

import Link from "next/link";
import type { ReactNode } from "react";

import { useAuth } from "@/lib/auth-context";
import { Brand } from "./brand";

export function AppShell({ children, nav }: { children: ReactNode; nav?: ReactNode }) {
  const { user, logout } = useAuth();
  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 px-6 py-4">
        <div className="glass mx-auto flex max-w-6xl items-center gap-4 rounded-2xl px-5 py-3">
          <Link href="/dashboard" aria-label="Turnstile home">
            <Brand size={28} withText />
          </Link>

          {nav && <div className="hidden flex-1 sm:block">{nav}</div>}

          {user && (
            <div className="ml-auto flex items-center gap-3">
              <span className="muted hidden text-sm lg:block">{user.email}</span>
              <button
                className="btn btn-ghost"
                style={{ padding: "0.45rem 0.9rem", fontSize: "0.85rem" }}
                onClick={logout}
              >
                Sign out
              </button>
            </div>
          )}
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
    </div>
  );
}
