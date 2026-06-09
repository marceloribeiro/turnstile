"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import type { ReactNode } from "react";

import { CenterSpinner, ErrorText, GlassCard, LiveBadge, Stat, usd, when } from "@/components/ui";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { useTelemetrySocket } from "@/lib/use-telemetry-socket";
import type { SessionRow } from "@/lib/types";

function Row({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-t border-white/10 py-2.5 text-sm">
      <span className="muted">{label}</span>
      <span className="text-right">{value}</span>
    </div>
  );
}

export default function SessionDetailPage() {
  const { token } = useAuth();
  const params = useParams<{ id: string; sessionId: string }>();
  const orgId = params.id;
  const sessionId = params.sessionId;
  const [s, setS] = useState<SessionRow | null>(null);
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    if (!token) return;
    try {
      setS(await api<SessionRow>(`/organizations/${orgId}/sessions/${sessionId}`, { token }));
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Failed to load session");
    }
  }, [token, orgId, sessionId]);

  useEffect(() => {
    void (async () => {
      await load();
    })();
  }, [load]);

  // Live updates: refetch this session whenever the org ingests telemetry.
  const live = useTelemetrySocket(orgId, token, load);

  if (err) {
    return (
      <div className="flex flex-col gap-4">
        <ErrorText>{err}</ErrorText>
        <Link href={`/organizations/${orgId}`} className="link text-sm">
          ← Back to sessions
        </Link>
      </div>
    );
  }
  if (!s) return <CenterSpinner />;

  const totalTokens = s.prompt_tokens + s.completion_tokens;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <Link href={`/organizations/${orgId}`} className="muted text-sm hover:text-white">
          ← Overview
        </Link>
        <div className="mt-1 flex items-center justify-between gap-3">
          <h2 className="break-all font-mono text-xl font-semibold tracking-tight">{s.session_key}</h2>
          <LiveBadge live={live} />
        </div>
        <p className="muted mt-1 text-sm">
          Per-session totals across {s.requests} request{s.requests === 1 ? "" : "s"}. Turnstile stores
          aggregates, not individual requests.
        </p>
      </div>

      <GlassCard strong className="p-6">
        <div className="grid grid-cols-2 gap-6 sm:grid-cols-4">
          <Stat label="Requests" value={s.requests.toLocaleString()} />
          <Stat label="Total cost" value={usd(s.cost)} />
          <Stat label="Dollars prevented" value={usd(s.prevented)} accent />
          <Stat label="Blocks" value={s.blocks.toLocaleString()} />
        </div>
      </GlassCard>

      <GlassCard className="p-6">
        <h3 className="mb-2 text-lg font-semibold">Breakdown</h3>
        <Row label="Model" value={s.model ?? "—"} />
        <Row label="Source" value={<span className="chip">{s.source}</span>} />
        <Row label="User" value={s.user_label ?? "—"} />
        <Row label="Prompt tokens" value={<span className="tnum">{s.prompt_tokens.toLocaleString()}</span>} />
        <Row
          label="Completion tokens"
          value={<span className="tnum">{s.completion_tokens.toLocaleString()}</span>}
        />
        <Row label="Total tokens" value={<span className="tnum">{totalTokens.toLocaleString()}</span>} />
        <Row label="First request" value={when(s.first_seen_at)} />
        <Row label="Last request" value={when(s.last_seen_at)} />
      </GlassCard>
    </div>
  );
}
