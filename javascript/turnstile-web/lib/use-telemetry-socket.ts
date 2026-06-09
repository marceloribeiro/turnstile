"use client";

import { useEffect, useRef, useState } from "react";

/**
 * Subscribes to an organization's live telemetry channel over WebSocket (backed
 * by Redis pub/sub on the API). Calls `onUpdate` whenever new telemetry is
 * ingested, so the dashboard refreshes without polling. Returns the live state.
 */
export function useTelemetrySocket(
  orgId: string,
  token: string | null,
  onUpdate: () => void,
): boolean {
  const cb = useRef(onUpdate);
  useEffect(() => {
    cb.current = onUpdate;
  }, [onUpdate]);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!token || !orgId) return;
    const httpBase = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";
    const wsBase = httpBase.replace(/^http/, "ws"); // http→ws, https→wss
    const url = `${wsBase}/ws/organizations/${orgId}?token=${encodeURIComponent(token)}`;

    // new WebSocket() can throw synchronously on a malformed URL; connection
    // failures arrive asynchronously via onerror/onclose instead.
    let ws: WebSocket | null = null;
    try {
      ws = new WebSocket(url);
      ws.onopen = () => setConnected(true);
      ws.onmessage = () => cb.current();
      ws.onclose = () => setConnected(false);
      ws.onerror = () => setConnected(false);
    } catch {
      // already disconnected; nothing to reset
    }
    return () => {
      setConnected(false);
      ws?.close();
    };
  }, [orgId, token]);

  return connected;
}
