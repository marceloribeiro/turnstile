"use client";

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
import type { Invitation, Member } from "@/lib/types";

export default function MembersPage() {
  const { token } = useAuth();
  const orgId = useParams<{ id: string }>().id;
  const [members, setMembers] = useState<Member[] | null>(null);
  const [invites, setInvites] = useState<Invitation[]>([]);

  const load = useCallback(async () => {
    if (!token) return;
    const [m, inv] = await Promise.all([
      api<Member[]>(`/organizations/${orgId}/members`, { token }),
      api<Invitation[]>(`/organizations/${orgId}/invitations`, { token }).catch(() => []),
    ]);
    setMembers(m);
    setInvites(inv);
  }, [token, orgId]);

  useEffect(() => {
    void (async () => {
      await load();
    })();
  }, [load]);

  if (members === null) return <CenterSpinner />;

  return (
    <div className="flex flex-col gap-6">
      <InviteForm orgId={orgId} token={token} onChange={load} />

      <GlassCard className="p-6">
        <SectionHeader title="Members" subtitle={`${members.length} in this organization`} />
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="muted text-left text-xs uppercase tracking-wide">
                <th className="pb-2 font-semibold">Name</th>
                <th className="pb-2 font-semibold">Email</th>
                <th className="pb-2 text-right font-semibold">Role</th>
              </tr>
            </thead>
            <tbody>
              {members.map((m) => (
                <tr key={m.id} className="border-t border-white/10">
                  <td className="py-2.5 font-medium">
                    {m.first_name} {m.last_name}
                  </td>
                  <td className="muted py-2.5">{m.email}</td>
                  <td className="py-2.5 text-right">
                    <span className="chip">{m.role}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </GlassCard>

      {invites.length > 0 && (
        <GlassCard className="p-6">
          <SectionHeader title="Pending invitations" subtitle="Awaiting acceptance" />
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="muted text-left text-xs uppercase tracking-wide">
                  <th className="pb-2 font-semibold">Email</th>
                  <th className="pb-2 font-semibold">Role</th>
                  <th className="pb-2 text-right font-semibold">Status</th>
                </tr>
              </thead>
              <tbody>
                {invites.map((i) => (
                  <tr key={i.id} className="border-t border-white/10">
                    <td className="py-2.5">{i.email}</td>
                    <td className="py-2.5">
                      <span className="chip">{i.role}</span>
                    </td>
                    <td className="py-2.5 text-right">
                      <span className="chip">{i.status}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </GlassCard>
      )}
    </div>
  );
}

function InviteForm({
  orgId,
  token,
  onChange,
}: {
  orgId: string;
  token: string | null;
  onChange: () => void;
}) {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("member");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");

  async function invite(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    setMsg("");
    setBusy(true);
    try {
      await api(`/organizations/${orgId}/invitations`, {
        method: "POST",
        body: { email, role },
        token,
      });
      setMsg(`Invitation sent to ${email}`);
      setEmail("");
      onChange();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Failed to invite");
    } finally {
      setBusy(false);
    }
  }

  return (
    <GlassCard strong className="p-6">
      <SectionHeader title="Invite a member" />
      {err && <div className="mb-3"><ErrorText>{err}</ErrorText></div>}
      {msg && <p className="mb-3 text-sm text-emerald-300">{msg}</p>}
      <form onSubmit={invite} className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <Field label="Email">
            <Input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="teammate@company.com"
            />
          </Field>
        </div>
        <Field label="Role">
          <select className="glass-input" value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="member">Member</option>
            <option value="admin">Admin</option>
            <option value="owner">Owner</option>
          </select>
        </Field>
        <Button type="submit" loading={busy}>
          Send invitation
        </Button>
      </form>
    </GlassCard>
  );
}
