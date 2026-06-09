"use client";

import { useRouter } from "next/navigation";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

import { api } from "./api";
import type { User } from "./types";

const KEY = "turnstile_token";

type AuthValue = {
  user: User | null;
  token: string | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (
    first_name: string,
    last_name: string,
    email: string,
    password: string,
  ) => Promise<void>;
  logout: () => void;
};

const Ctx = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    const t = typeof window !== "undefined" ? localStorage.getItem(KEY) : null;
    // Resolve the stored session asynchronously so no state update runs
    // synchronously inside the effect body (avoids cascading renders).
    void (async () => {
      if (!t) {
        setLoading(false);
        return;
      }
      setToken(t);
      try {
        setUser(await api<User>("/me", { token: t }));
      } catch {
        localStorage.removeItem(KEY);
        setToken(null);
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const persist = useCallback((t: string, u: User) => {
    localStorage.setItem(KEY, t);
    setToken(t);
    setUser(u);
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const r = await api<{ user: User; token: string }>("/login", {
        method: "POST",
        body: { email, password },
      });
      persist(r.token, r.user);
    },
    [persist],
  );

  const register = useCallback(
    async (first_name: string, last_name: string, email: string, password: string) => {
      const r = await api<{ user: User; token: string }>("/register", {
        method: "POST",
        body: { first_name, last_name, email, password },
      });
      persist(r.token, r.user);
    },
    [persist],
  );

  const logout = useCallback(() => {
    localStorage.removeItem(KEY);
    setToken(null);
    setUser(null);
    router.push("/login");
  }, [router]);

  return (
    <Ctx.Provider value={{ user, token, loading, login, register, logout }}>
      {children}
    </Ctx.Provider>
  );
}

export function useAuth(): AuthValue {
  const c = useContext(Ctx);
  if (!c) throw new Error("useAuth must be used within AuthProvider");
  return c;
}

/** Redirect to /login when not authenticated. */
export function useRequireAuth() {
  const { user, loading } = useAuth();
  const router = useRouter();
  useEffect(() => {
    if (!loading && !user) router.replace("/login");
  }, [loading, user, router]);
  return { user, loading };
}
