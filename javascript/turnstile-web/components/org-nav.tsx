"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const TABS = [
  { label: "Overview", seg: "" },
  { label: "Projects", seg: "projects" },
  { label: "Members", seg: "members" },
];

export function OrgNav({ orgId }: { orgId: string }) {
  const pathname = usePathname();
  const base = `/organizations/${orgId}`;
  return (
    <nav className="flex items-center gap-1">
      {TABS.map((t) => {
        const href = t.seg ? `${base}/${t.seg}` : base;
        // active for the section (so a project detail page keeps "Projects" lit)
        const active = t.seg ? pathname.startsWith(href) : pathname === href;
        return (
          <Link
            key={t.label}
            href={href}
            className={`rounded-lg px-3 py-1.5 text-sm font-semibold transition ${
              active ? "bg-white/15 text-white" : "text-white/55 hover:bg-white/5 hover:text-white"
            }`}
          >
            {t.label}
          </Link>
        );
      })}
    </nav>
  );
}
