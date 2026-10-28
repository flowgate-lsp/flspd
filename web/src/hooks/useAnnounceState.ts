import { useState, useEffect, useMemo, useCallback } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI } from '../lib/api';
import type { AnnounceAction, AnnounceState } from '../lib/announce';

// The node's NIP-LSP announcement status and the Pause/Resume/Unlist
// actions (T8-2).
export const useAnnounceState = () => {
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
      setState(await api.getAnnounceState());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [api]);

  const act = async (action: AnnounceAction) => {
    try {
      setBusy(true);
      setError(null);
      setState(await api.setAnnounceState(action));
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
      setState(await api.recheckAnnouncement());
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
