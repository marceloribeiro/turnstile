import type { ReactNode } from "react";

import { Brand } from "./brand";
import { GlassCard } from "./ui";

export function AuthLayout({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-screen items-center justify-center px-6 py-10">
      <div className="w-full max-w-md">
        <div className="mb-8 flex flex-col items-center gap-3 text-center">
          <Brand size={46} />
          <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
          <p className="muted">{subtitle}</p>
        </div>
        <GlassCard strong className="p-8">
          {children}
        </GlassCard>
      </div>
    </div>
  );
}
