import { useState, useEffect, useMemo } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI } from '../lib/api';

export const useSettings = () => {
  const { user, getAuthHeader } = useAuth();
  const [config, setConfig] = useState<any>(null);
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
      const data = await api.getConfig();
      setConfig(data.config);
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
      await api.updateConfig(newConfig);
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
    if (user) {
      refreshConfig();
    }
  }, [user]);

  return { config, loading, saving, error, refreshConfig, updateConfig };
};
