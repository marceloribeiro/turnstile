"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { CopyButton } from "@/components/copy-button";
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
import { useAuth } from "@/lib/auth-context";
import { useTelemetrySocket } from "@/lib/use-telemetry-socket";
import type { Deployment, Project, SessionRow, Summary } from "@/lib/types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";

function envSnippet(key: string): string {
  return `TURNSTILE_INGEST_URL=${API_BASE}/ingest/sessions\nTURNSTILE_INGEST_KEY=${key}`;
}

export default function ProjectPage() {
  const { token } = useAuth();
  const params = useParams<{ id: string; projectId: string }>();
  const orgId = params.id;
  const projectId = params.projectId;

  const [project, setProject] = useState<Project | null>(null);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [sessions, setSessions] = useState<SessionRow[]>([]);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [err, setErr] = useState("");

  const base = `/organizations/${orgId}/projects/${projectId}`;

  const load = useCallback(async () => {
    if (!token) return;
    try {
      const [p, s, ss, d] = await Promise.all([
        api<Project>(base, { token }),
        api<Summary>(`${base}/summary`, { token }),
        api<SessionRow[]>(`${base}/sessions`, { token }),
        api<Deployment[]>(`${base}/deployments`, { token }),
      ]);
      setProject(p);
      setSummary(s);
      setSessions(ss);
      setDeployments(d);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Failed to load");
    }
  }, [token, base]);

  useEffect(() => {
    void (async () => {
      await load();
    })();
  }, [load]);

  // live updates arrive org-wide; refetch (cheap) keeps this project's view fresh
  useTelemetrySocket(orgId, token, load);

  if (err)
    return (
      <>
        <ErrorText>{err}</ErrorText>
        <Link href={`/organizations/${orgId}/projects`} className="muted mt-4 inline-block text-sm">
          ← Projects
        </Link>
      </>
    );
  if (!project || !summary) return <CenterSpinner />;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <Link href={`/organizations/${orgId}/projects`} className="muted text-sm hover:text-white">
          ← Projects
        </Link>
        <h2 className="mt-1 text-2xl font-bold tracking-tight">{project.name}</h2>
        {project.description && <p className="muted mt-1 text-sm">{project.description}</p>}
      </div>

      <GlassCard strong className="p-8">
        <div className="grid items-center gap-8 md:grid-cols-[1.3fr_2fr]">
          <Stat label="Dollars prevented" value={usd(summary.dollars_prevented)} accent />
          <div className="grid grid-cols-2 gap-6 sm:grid-cols-4">
            <Stat label="Sessions" value={summary.sessions} />
            <Stat label="Requests" value={summary.requests} />
            <Stat label="Total cost" value={usd(summary.total_cost)} />
            <Stat label="Blocks" value={summary.blocks} />
          </div>
        </div>
      </GlassCard>

      <div className="grid gap-6 lg:grid-cols-3">
        <GlassCard className="p-6 lg:col-span-2">
          <h3 className="mb-4 text-lg font-semibold">Sessions</h3>
          {sessions.length === 0 ? (
            <p className="muted text-sm">No telemetry yet for this project.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="muted text-left text-xs uppercase tracking-wide">
                    <th className="pb-2 font-semibold">Session</th>
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

        <DeploymentsCard orgId={orgId} projectId={projectId} token={token} deployments={deployments} onChange={load} />
      </div>
    </div>
  );
}

function DeploymentsCard({
  orgId,
  projectId,
  token,
  deployments,
  onChange,
}: {
  orgId: string;
  projectId: string;
  token: string | null;
  deployments: Deployment[];
  onChange: () => void;
}) {
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [newKey, setNewKey] = useState<string | null>(null);
  const [err, setErr] = useState("");

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    setBusy(true);
    try {
      const d = await api<Deployment>(
        `/organizations/${orgId}/projects/${projectId}/deployments`,
        { method: "POST", body: { name }, token },
      );
      setNewKey(d.ingest_key ?? null);
      setName("");
      await onChange();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Failed to create");
    } finally {
      setBusy(false);
    }
  }

  return (
    <GlassCard strong className="h-fit p-6">
      <SectionHeader title="Ingest keys" subtitle="One per Turnstile container in this project." />
      <ul className="mb-4 flex flex-col gap-2">
        {deployments.map((d) => (
          <li key={d.id} className="flex items-center justify-between text-sm">
            <span>{d.name}</span>
            <span className="muted font-mono text-xs">{d.ingest_key_prefix}…</span>
          </li>
        ))}
      </ul>

      {newKey && (
        <div className="mb-4 rounded-xl border border-emerald-400/30 bg-emerald-500/10 p-3">
          <div className="mb-1 flex items-center justify-between gap-2">
            <p className="text-xs font-semibold text-emerald-200">Copy it now — shown once:</p>
            <CopyButton value={newKey} label="Copy key" />
          </div>
          <code className="block break-all font-mono text-xs text-emerald-100">{newKey}</code>

          <div className="mb-1 mt-3 flex items-center justify-between gap-2">
            <p className="text-xs font-semibold text-white/55">Container .env</p>
            <CopyButton value={envSnippet(newKey)} label="Copy .env" />
          </div>
          <pre className="glass-input overflow-x-auto whitespace-pre text-[11px] text-white/80">
{envSnippet(newKey)}
          </pre>
        </div>
      )}

      <form onSubmit={create} className="flex flex-col gap-3">
        <ErrorText>{err}</ErrorText>
        <Field label="New ingest key">
          <Input required value={name} onChange={(e) => setName(e.target.value)} placeholder="prod-us-east" />
        </Field>
        <Button type="submit" variant="ghost" loading={busy}>
          Create &amp; mint key
        </Button>
      </form>
    </GlassCard>
  );
}
