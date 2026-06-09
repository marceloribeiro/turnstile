"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { AppShell } from "@/components/app-shell";
import {
  Button,
  CenterSpinner,
  ErrorText,
  Field,
  GlassCard,
  Input,
  SectionHeader,
  Stat,
  usd,
} from "@/components/ui";
import { api, ApiError } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/lib/auth-context";
import type { Org, Usage } from "@/lib/types";

export default function Dashboard() {
  const { user, loading } = useRequireAuth();
  const { token } = useAuth();
  const [orgs, setOrgs] = useState<Org[] | null>(null);
  const [usage, setUsage] = useState<Usage | null>(null);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  const [err, setErr] = useState("");

  const refresh = useCallback(async () => {
    if (!token) return;
    const [o, u] = await Promise.all([
      api<Org[]>("/organizations", { token }),
      api<Usage>("/me/usage", { token }),
    ]);
    setOrgs(o);
    setUsage(u);
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
        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
        <p className="muted mt-1">Spend across all your Turnstile organizations.</p>
      </div>

      {usage === null || orgs === null ? (
        <CenterSpinner />
      ) : (
        <div className="flex flex-col gap-6">
          <GlassCard className="p-6">
            <div className="grid grid-cols-2 gap-6 lg:grid-cols-4">
              <Stat label="Total spent" value={usd(usage.total_cost)} />
              <Stat label="Dollars prevented" value={usd(usage.dollars_prevented)} accent />
              <Stat label="Requests" value={usage.requests.toLocaleString()} />
              <Stat label="Sessions" value={usage.sessions.toLocaleString()} />
            </div>
          </GlassCard>

          <GlassCard className="p-6">
            <SectionHeader title="Spend by model" subtitle="Across all your organizations" />
            {usage.by_model.length === 0 ? (
              <p className="muted text-sm">
                No usage yet. Point a Turnstile data plane at one of your orgs to see spend here.
              </p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="muted text-left text-xs uppercase tracking-wide">
                      <th className="pb-2 font-semibold">Model</th>
                      <th className="pb-2 text-right font-semibold">Requests</th>
                      <th className="pb-2 text-right font-semibold">Tokens</th>
                      <th className="pb-2 text-right font-semibold">Spend</th>
                    </tr>
                  </thead>
                  <tbody>
                    {usage.by_model.map((m) => (
                      <tr key={m.model} className="border-t border-white/10">
                        <td className="py-2.5 font-medium">{m.model}</td>
                        <td className="tnum muted py-2.5 text-right">
                          {m.requests.toLocaleString()}
                        </td>
                        <td className="tnum muted py-2.5 text-right">
                          {(m.prompt_tokens + m.completion_tokens).toLocaleString()}
                        </td>
                        <td className="tnum py-2.5 text-right font-semibold">{usd(m.cost)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </GlassCard>

          <GlassCard strong className="p-6">
            <SectionHeader title="New organization" />
            <form onSubmit={create} className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="flex-1">
                <Field label="Name">
                  <Input
                    required
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Acme Inc."
                  />
                </Field>
              </div>
              <Button type="submit" loading={creating}>
                Create organization
              </Button>
            </form>
            {err && (
              <div className="mt-3">
                <ErrorText>{err}</ErrorText>
              </div>
            )}
          </GlassCard>

          <GlassCard className="p-6">
            <SectionHeader
              title="Your organizations"
              subtitle={`${orgs.length} ${orgs.length === 1 ? "organization" : "organizations"}`}
            />
            {orgs.length === 0 ? (
              <p className="muted text-sm">No organizations yet — create your first one above.</p>
            ) : (
              <ul className="flex flex-col divide-y divide-white/10">
                {orgs.map((o) => (
                  <li key={o.id} className="flex items-center justify-between gap-4 py-3">
                    <div>
                      <div className="font-semibold">{o.name}</div>
                      <div className="muted text-xs">
                        Created {new Date(o.created_at).toLocaleDateString()}
                      </div>
                    </div>
                    <Link href={`/organizations/${o.id}`} className="btn btn-ghost">
                      Open →
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </GlassCard>
        </div>
      )}
    </AppShell>
  );
}
