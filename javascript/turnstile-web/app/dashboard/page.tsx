"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { AppShell } from "@/components/app-shell";
import { Button, CenterSpinner, ErrorText, Field, GlassCard, Input } from "@/components/ui";
import { api, ApiError } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/lib/auth-context";
import type { Org } from "@/lib/types";

export default function Dashboard() {
  const { user, loading } = useRequireAuth();
  const { token } = useAuth();
  const [orgs, setOrgs] = useState<Org[] | null>(null);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  const [err, setErr] = useState("");

  const refresh = useCallback(async () => {
    if (!token) return;
    setOrgs(await api<Org[]>("/organizations", { token }));
  }, [token]);

  useEffect(() => {
    void (async () => {
      await refresh();
    })();
  }, [refresh]);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    setCreating(true);
    try {
      await api("/organizations", { method: "POST", body: { name }, token });
      setName("");
      await refresh();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Failed to create");
    } finally {
      setCreating(false);
    }
  }

  if (loading || !user) return <CenterSpinner />;

  return (
    <AppShell>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">Organizations</h1>
        <p className="muted mt-1">Your teams and their Turnstile deployments.</p>
      </div>

      <div className="grid gap-6 md:grid-cols-3">
        <div className="md:col-span-2">
          {orgs === null ? (
            <CenterSpinner />
          ) : orgs.length === 0 ? (
            <GlassCard className="muted p-8 text-center">
              No organizations yet — create your first one to get started.
            </GlassCard>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2">
              {orgs.map((o) => (
                <Link key={o.id} href={`/organizations/${o.id}`}>
                  <GlassCard className="h-full cursor-pointer p-6 transition hover:scale-[1.02]">
                    <div className="text-lg font-semibold">{o.name}</div>
                    {o.website_url && <div className="muted mt-1 text-sm">{o.website_url}</div>}
                    <div className="muted mt-6 text-xs">
                      Created {new Date(o.created_at).toLocaleDateString()}
                    </div>
                  </GlassCard>
                </Link>
              ))}
            </div>
          )}
        </div>

        <GlassCard strong className="h-fit p-6">
          <h2 className="mb-4 text-lg font-semibold">New organization</h2>
          <form onSubmit={create} className="flex flex-col gap-3">
            <ErrorText>{err}</ErrorText>
            <Field label="Name">
              <Input required value={name} onChange={(e) => setName(e.target.value)} placeholder="Acme Inc." />
            </Field>
            <Button type="submit" loading={creating}>
              Create organization
            </Button>
          </form>
        </GlassCard>
      </div>
    </AppShell>
  );
}
