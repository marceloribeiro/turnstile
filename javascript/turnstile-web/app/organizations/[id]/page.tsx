"use client";

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { CenterSpinner, GlassCard, LiveBadge, Stat, usd } from "@/components/ui";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { useTelemetrySocket } from "@/lib/use-telemetry-socket";
import type { Project, SessionRow, Summary } from "@/lib/types";

export default function OverviewPage() {
  const { token } = useAuth();
  const orgId = useParams<{ id: string }>().id;
  const [summary, setSummary] = useState<Summary | null>(null);
  const [sessions, setSessions] = useState<SessionRow[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectFilter, setProjectFilter] = useState("");

  const load = useCallback(async () => {
    if (!token) return;
    const q = projectFilter ? `?project_id=${projectFilter}` : "";
    const [s, ss, ps] = await Promise.all([
      api<Summary>(`/organizations/${orgId}/summary${q}`, { token }),
      api<SessionRow[]>(`/organizations/${orgId}/sessions${q}`, { token }),
      api<Project[]>(`/organizations/${orgId}/projects`, { token }),
    ]);
    setSummary(s);
    setSessions(ss);
    setProjects(ps);
  }, [token, orgId, projectFilter]);

  useEffect(() => {
    void (async () => {
      await load();
    })();
  }, [load]);

  const live = useTelemetrySocket(orgId, token, load);

  if (!summary) return <CenterSpinner />;

  return (
    <div className="flex flex-col gap-6">
      <GlassCard strong className="p-8">
        <div className="mb-5 flex items-center justify-between gap-3">
          <span className="text-xs font-semibold uppercase tracking-wide text-white/50">
            Live overview
          </span>
          <div className="flex items-center gap-3">
            {projects.length > 0 && (
              <select
                className="glass-input"
                style={{ width: "auto", padding: "0.4rem 0.7rem", fontSize: "0.85rem" }}
                value={projectFilter}
                onChange={(e) => setProjectFilter(e.target.value)}
                aria-label="Filter by project"
              >
                <option value="">All projects</option>
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            )}
            <LiveBadge live={live} />
          </div>
        </div>
        <div className="grid items-center gap-8 md:grid-cols-[1.3fr_2fr]">
          <div>
            <Stat label="Dollars prevented" value={usd(summary.dollars_prevented)} accent />
            <p className="muted mt-2 text-sm">
              Saved by killing runaway loops and budget overruns.
            </p>
          </div>
          <div className="grid grid-cols-2 gap-6 sm:grid-cols-4">
            <Stat label="Sessions" value={summary.sessions} />
            <Stat label="Requests" value={summary.requests} />
            <Stat label="Total cost" value={usd(summary.total_cost)} />
            <Stat label="Blocks" value={summary.blocks} />
          </div>
        </div>
      </GlassCard>

      <GlassCard className="p-6">
        <h2 className="mb-4 text-lg font-semibold">Sessions</h2>
        {sessions.length === 0 ? (
          <p className="muted text-sm">
            No telemetry yet. Point a Turnstile container at this org&apos;s ingest key (Developer tab).
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="muted text-left text-xs uppercase tracking-wide">
                  <th className="pb-2 font-semibold">Session</th>
                  <th className="pb-2 font-semibold">Source</th>
                  <th className="pb-2 font-semibold">Model</th>
                  <th className="pb-2 text-right font-semibold">Reqs</th>
                  <th className="pb-2 text-right font-semibold">Cost</th>
                  <th className="pb-2 text-right font-semibold">Blocks</th>
                </tr>
              </thead>
              <tbody>
                {sessions.map((s) => (
                  <tr key={s.id} className="border-t border-white/10">
                    <td className="py-2 pr-2 font-mono text-xs">{s.session_key}</td>
                    <td className="py-2">
                      <span className="chip">{s.source}</span>
                    </td>
                    <td className="muted py-2 text-xs">{s.model ?? "—"}</td>
                    <td className="py-2 text-right">{s.requests}</td>
                    <td className="py-2 text-right">{usd(s.cost)}</td>
                    <td className="py-2 text-right">
                      {s.blocks > 0 ? (
                        <span className="text-rose-300">{s.blocks}</span>
                      ) : (
                        <span className="muted">0</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </GlassCard>
    </div>
  );
}
