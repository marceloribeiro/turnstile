"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import {
  Button,
  CenterSpinner,
  ErrorText,
  Field,
  GlassCard,
  Input,
  SectionHeader,
} from "@/components/ui";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { Project } from "@/lib/types";

export default function ProjectsPage() {
  const { token } = useAuth();
  const orgId = useParams<{ id: string }>().id;
  const [projects, setProjects] = useState<Project[] | null>(null);

  const load = useCallback(async () => {
    if (!token) return;
    setProjects(await api<Project[]>(`/organizations/${orgId}/projects`, { token }));
  }, [token, orgId]);

  useEffect(() => {
    void (async () => {
      await load();
    })();
  }, [load]);

  if (projects === null) return <CenterSpinner />;

  return (
    <div className="flex flex-col gap-6">
      <NewProjectForm orgId={orgId} token={token} onChange={load} />

      <GlassCard className="p-6">
        <SectionHeader
          title="Projects"
          subtitle={`${projects.length} ${projects.length === 1 ? "project" : "projects"} in this organization`}
        />
        {projects.length === 0 ? (
          <p className="muted text-sm">
            No projects yet — create one above to group ingest keys and attribute telemetry.
          </p>
        ) : (
          <ul className="flex flex-col divide-y divide-white/10">
            {projects.map((p) => (
              <li key={p.id} className="flex items-center justify-between gap-4 py-3">
                <div>
                  <div className="font-semibold">{p.name}</div>
                  {p.description && <div className="muted text-sm">{p.description}</div>}
                  <div className="muted text-xs">
                    Created {new Date(p.created_at).toLocaleDateString()}
                  </div>
                </div>
                <Link
                  href={`/organizations/${orgId}/projects/${p.id}`}
                  className="btn btn-ghost"
                >
                  Open →
                </Link>
              </li>
            ))}
          </ul>
        )}
      </GlassCard>
    </div>
  );
}

function NewProjectForm({
  orgId,
  token,
  onChange,
}: {
  orgId: string;
  token: string | null;
  onChange: () => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    setBusy(true);
    try {
      await api(`/organizations/${orgId}/projects`, {
        method: "POST",
        body: { name, description: description || null },
        token,
      });
      setName("");
      setDescription("");
      onChange();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Failed to create");
    } finally {
      setBusy(false);
    }
  }

  return (
    <GlassCard strong className="p-6">
      <SectionHeader title="New project" />
      {err && (
        <div className="mb-3">
          <ErrorText>{err}</ErrorText>
        </div>
      )}
      <form onSubmit={create} className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <Field label="Name">
            <Input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Checkout agents"
            />
          </Field>
        </div>
        <div className="flex-1">
          <Field label="Description (optional)">
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What this project covers"
            />
          </Field>
        </div>
        <Button type="submit" loading={busy}>
          Create project
        </Button>
      </form>
    </GlassCard>
  );
}
