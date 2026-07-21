import { useState, useEffect } from 'react';

export interface NodeStatus {
  name: string;
  connected: boolean;
}

export interface SystemInfo {
  version: string;
  commit: string;
  dashboard_enabled: boolean;
  nodes: NodeStatus[];
}

const POLL_INTERVAL_MS = 15000;

// Polls the backend's public, unauthenticated /info endpoint -- used both
// to decide whether the login form should even be shown (dashboard_enabled)
// and to keep a persistent node-status readout visible after login too.
export const useSystemInfo = () => {
  const [info, setInfo] = useState<SystemInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const fetchInfo = async () => {
      try {
        const resp = await fetch(`${window.location.origin}/info`);
        if (!resp.ok) throw new Error(`Server returned ${resp.status}`);
        const data: SystemInfo = await resp.json();
        if (!cancelled) {
          setInfo(data);
          setError(null);
        }
      } catch (err: any) {
        if (!cancelled) setError(err.message || 'Failed to reach backend');
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    fetchInfo();
    const interval = setInterval(fetchInfo, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  return { info, loading, error };
};
