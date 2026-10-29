import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI } from '../lib/api';
import type { AnnounceAction, AnnounceState } from '../lib/announce';

// The node's NIP-LSP announcement status and the Pause/Resume/Unlist
// actions (T8-2).
// node is the targeted fleet node; null waits for a choice. state is only
// ever the targeted node's: it is cleared when the node changes, and an
// answer for a node no longer targeted is dropped (T8-6, R4-F4).
export const useAnnounceState = (node: string | null) => {
  const { user, getAuthHeader } = useAuth();
  const [state, setState] = useState<AnnounceState | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const targeted = useRef(node);
  targeted.current = node;

  const api = useMemo(() => {
    const proxyUrl = (import.meta as { env?: Record<string, string> }).env?.VITE_PROXY_URL;
    return new DashboardAPI(window.location.origin, getAuthHeader, proxyUrl);
  }, [getAuthHeader]);

  // Runs call for the node targeted now and keeps its answer only while that
  // node is still the one targeted.
  const forNode = useCallback(async (call: (target: string) => Promise<AnnounceState>, need: boolean) => {
    const target = node;
    if (target === null) {
      if (need) setError('Choose a node first');
      return false;
    }
    try {
      setError(null);
      const next = await call(target);
      if (targeted.current === target) setState(next);
      return true;
    } catch (err) {
      if (targeted.current === target) setError(err instanceof Error ? err.message : String(err));
      return false;
    }
  }, [node]);

  const refresh = useCallback(
    () => forNode((target) => api.getAnnounceState(target), false),
    [forNode, api],
  );

  const act = async (action: AnnounceAction) => {
    setBusy(true);
    try {
      return await forNode((target) => api.setAnnounceState(action, target), true);
    } finally {
      setBusy(false);
    }
  };

  const recheck = async () => {
    setBusy(true);
    try {
      await forNode((target) => api.recheckAnnouncement(target), true);
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    setState(null);
    setError(null);
  }, [node]);

  useEffect(() => {
    if (user) {
      void refresh();
    }
  }, [user, refresh]);

  return { state, busy, error, refresh, act, recheck };
};
