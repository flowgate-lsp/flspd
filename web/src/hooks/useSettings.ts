import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI } from '../lib/api';

// node is the targeted fleet node; null waits for a choice. config is only
// ever the targeted node's: it is cleared when the node changes, and an
// answer for a node no longer targeted is dropped (T8-6, R4-F3/R4-F5).
export const useSettings = (node: string | null) => {
  const { user, getAuthHeader } = useAuth();
  const [config, setConfig] = useState<any>(null);
  // NOSTR_ANNOUNCE_RELAYS, read-only (server env), for the announce card.
  const [announceRelays, setAnnounceRelays] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const targeted = useRef(node);
  targeted.current = node;

  const api = useMemo(() => {
    const proxyUrl = (import.meta as any).env?.VITE_PROXY_URL;
    return new DashboardAPI(window.location.origin, getAuthHeader, proxyUrl);
  }, [getAuthHeader]);

  const refreshConfig = useCallback(async () => {
    const target = node;
    if (target === null) return;
    try {
      setLoading(true);
      setError(null);
      const data = await api.getConfig(target);
      if (targeted.current !== target) return;
      setConfig(data.config);
      setAnnounceRelays(Array.isArray(data.announce_relays) ? data.announce_relays : []);
    } catch (err: any) {
      if (targeted.current === target) setError(err.message);
    } finally {
      if (targeted.current === target) setLoading(false);
    }
  }, [api, node]);

  const updateConfig = async (newConfig: any) => {
    const target = node;
    if (target === null || config === null) {
      setError(target === null ? 'Choose a node first' : "The node's settings have not loaded");
      return false;
    }
    try {
      setSaving(true);
      setError(null);
      await api.updateConfig(newConfig, target);
      if (targeted.current === target) setConfig(newConfig);
      return true;
    } catch (err: any) {
      if (targeted.current === target) setError(err.message);
      return false;
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    setConfig(null);
    setAnnounceRelays([]);
    setError(null);
  }, [node]);

  useEffect(() => {
    if (user && node !== null) {
      void refreshConfig();
    }
  }, [user, node, refreshConfig]);

  return { config, announceRelays, loading, saving, error, refreshConfig, updateConfig };
};
