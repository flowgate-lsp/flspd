import { useState, useEffect, useMemo, useCallback } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI } from '../lib/api';
import type { AnnounceAction, AnnounceState } from '../lib/announce';

// The node's NIP-LSP announcement status and the Pause/Resume/Unlist
// actions (T8-2).
// node is the targeted fleet node; null waits for a choice.
export const useAnnounceState = (node: string | null) => {
  const { user, getAuthHeader } = useAuth();
  const [state, setState] = useState<AnnounceState | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => {
    const proxyUrl = (import.meta as { env?: Record<string, string> }).env?.VITE_PROXY_URL;
    return new DashboardAPI(window.location.origin, getAuthHeader, proxyUrl);
  }, [getAuthHeader]);

  const refresh = useCallback(async () => {
    try {
      setError(null);
      if (!node) return;
      setState(await api.getAnnounceState(node));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [api, node]);

  const act = async (action: AnnounceAction) => {
    try {
      setBusy(true);
      setError(null);
      if (!node) throw new Error('Choose a node first');
      setState(await api.setAnnounceState(action, node));
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      return false;
    } finally {
      setBusy(false);
    }
  };

  const recheck = async () => {
    try {
      setBusy(true);
      setError(null);
      if (!node) throw new Error('Choose a node first');
      setState(await api.recheckAnnouncement(node));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    if (user) {
      void refresh();
    }
  }, [user, refresh]);

  return { state, busy, error, refresh, act, recheck };
};
