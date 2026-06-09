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
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    if (!token) return;
    setProjects(await api<Project[]>(`/organizations/${orgId}/projects`, { token }));
  }, [token, orgId]);

  useEffect(() => {
    void (async () => {
      await load();
    })();
  }, [load]);

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
      await load();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Failed to create");
    } finally {
      setBusy(false);
    }
  }

  if (projects === null) return <CenterSpinner />;

  return (
    <div className="grid gap-6 md:grid-cols-3">
      <div className="md:col-span-2">
        {projects.length === 0 ? (
          <GlassCard className="muted p-8 text-center">
            No projects yet — create one to group ingest keys and attribute telemetry.
          </GlassCard>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2">
            {projects.map((p) => (
              <Link key={p.id} href={`/organizations/${orgId}/projects/${p.id}`}>
                <GlassCard className="h-full cursor-pointer p-6 transition hover:scale-[1.02]">
                  <div className="text-lg font-semibold">{p.name}</div>
                  {p.description && <div className="muted mt-1 text-sm">{p.description}</div>}
                  <div className="muted mt-6 text-xs">
                    Created {new Date(p.created_at).toLocaleDateString()}
                  </div>
                </GlassCard>
              </Link>
            ))}
          </div>
        )}
      </div>

      <GlassCard strong className="h-fit p-6">
        <SectionHeader title="New project" />
        <form onSubmit={create} className="flex flex-col gap-3">
          <ErrorText>{err}</ErrorText>
          <Field label="Name">
            <Input required value={name} onChange={(e) => setName(e.target.value)} placeholder="Checkout agents" />
          </Field>
          <Field label="Description (optional)">
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What this project covers" />
          </Field>
          <Button type="submit" loading={busy}>
            Create project
          </Button>
        </form>
      </GlassCard>
    </div>
  );
}
