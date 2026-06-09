"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";

import { AuthLayout } from "@/components/auth-layout";
import { Button } from "@/components/ui";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

function AcceptInner() {
  const sp = useSearchParams();
  const invToken = sp.get("token") ?? "";
  const { token, user, loading } = useAuth();
  const router = useRouter();
  const [status, setStatus] = useState<"idle" | "working" | "done" | "error">("idle");
  const [msg, setMsg] = useState("");

  async function accept() {
    setStatus("working");
    try {
      await api("/invitations/accept", { method: "POST", body: { token: invToken }, token });
      setStatus("done");
      setTimeout(() => router.push("/dashboard"), 1200);
    } catch (e) {
      setStatus("error");
      setMsg(e instanceof ApiError ? e.message : "Failed to accept");
    }
  }

  if (loading) return <p className="muted text-center">Loading…</p>;
  if (!user)
    return (
      <div className="text-center">
        <p className="muted mb-4">Sign in to accept this invitation.</p>
        <Link href="/login" className="btn btn-primary inline-flex">
          Sign in
        </Link>
      </div>
    );
  if (!invToken) return <p className="muted text-center">Missing invitation token.</p>;
  if (status === "done")
    return <p className="text-center text-emerald-300">Joined! Redirecting…</p>;

  return (
    <div className="flex flex-col gap-4 text-center">
      {status === "error" && <p className="text-sm text-rose-300">{msg}</p>}
      <p className="muted">You&apos;ve been invited to join an organization on Turnstile.</p>
      <Button onClick={accept} loading={status === "working"}>
        Accept invitation
      </Button>
    </div>
  );
}

export default function AcceptPage() {
  return (
    <AuthLayout title="Accept invitation" subtitle="Join your team on Turnstile">
      <Suspense fallback={<p className="muted text-center">Loading…</p>}>
        <AcceptInner />
      </Suspense>
    </AuthLayout>
  );
}
