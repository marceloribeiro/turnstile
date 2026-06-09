"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { AppShell } from "@/components/app-shell";
import { OrgNav } from "@/components/org-nav";
import { CenterSpinner, ErrorText } from "@/components/ui";
import { api, ApiError } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/lib/auth-context";
import type { Org } from "@/lib/types";

export default function OrgLayout({ children }: { children: React.ReactNode }) {
  const { user, loading } = useRequireAuth();
  const { token } = useAuth();
  const orgId = useParams<{ id: string }>().id;
  const [org, setOrg] = useState<Org | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (!token) return;
    api<Org>(`/organizations/${orgId}`, { token })
      .then(setOrg)
      .catch((e) => setErr(e instanceof ApiError ? e.message : "Failed to load"));
  }, [token, orgId]);

  return (
    <AppShell nav={<OrgNav orgId={orgId} />}>
      {loading || !user ? (
        <CenterSpinner />
      ) : (
        <>
          <div className="mb-2">
            <Link href="/dashboard" className="muted text-sm hover:text-white">
              ← Organizations
            </Link>
          </div>

          {err ? (
            <ErrorText>{err}</ErrorText>
          ) : (
            <>
              <h1 className="mb-8 text-3xl font-bold tracking-tight">{org?.name ?? "…"}</h1>
              {children}
            </>
          )}
        </>
      )}
    </AppShell>
  );
}
