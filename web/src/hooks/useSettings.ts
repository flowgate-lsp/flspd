import { useState, useEffect, useMemo } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI } from '../lib/api';

// node is the targeted fleet node; null waits for a choice.
export const useSettings = (node: string | null) => {
  const { user, getAuthHeader } = useAuth();
  const [config, setConfig] = useState<any>(null);
  // NOSTR_ANNOUNCE_RELAYS, read-only (server env), for the announce card.
  const [announceRelays, setAnnounceRelays] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => {
    const proxyUrl = (import.meta as any).env?.VITE_PROXY_URL;
    return new DashboardAPI(window.location.origin, getAuthHeader, proxyUrl);
  }, [getAuthHeader]);

  const refreshConfig = async () => {
    try {
      setLoading(true);
      if (!node) return;
      const data = await api.getConfig(node);
      setConfig(data.config);
      setAnnounceRelays(Array.isArray(data.announce_relays) ? data.announce_relays : []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const updateConfig = async (newConfig: any) => {
    try {
      setSaving(true);
      setError(null);
      if (!node) throw new Error('Choose a node first');
      await api.updateConfig(newConfig, node);
      setConfig(newConfig);
      return true;
    } catch (err: any) {
      setError(err.message);
      return false;
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    if (user && node) {
      refreshConfig();
    }
  }, [user, node]);

  return { config, announceRelays, loading, saving, error, refreshConfig, updateConfig };
};
