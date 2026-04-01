import { useState, useEffect, useMemo } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI } from '../lib/api';

export const useDashboard = () => {
  const { user, getAuthHeader } = useAuth();
  const [summary, setSummary] = useState<any>(null);
  const [orders, setOrders] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => {
    const proxyUrl = (import.meta as any).env?.VITE_PROXY_URL;
    return new DashboardAPI(window.location.origin, getAuthHeader, proxyUrl);
  }, [getAuthHeader]);

  const refreshSummary = async () => {
    try {
      const data = await api.getSummary();
      setSummary(data);
    } catch (err: any) {
      setError(err.message);
    }
  };

  const refreshOrders = async () => {
    try {
      const data = await api.listOrders();
      setOrders(data);
    } catch (err: any) {
      setError(err.message);
    }
  };

  useEffect(() => {
    if (!user) return;
    
    setLoading(true);
    setError(null);

    Promise.all([refreshSummary(), refreshOrders()])
      .finally(() => setLoading(false));

    // Refresh every 30 seconds
    const interval = setInterval(() => {
      refreshSummary();
      refreshOrders();
    }, 30000);

    return () => clearInterval(interval);
  }, [user]);

  return { api, summary, orders, loading, error, refreshSummary, refreshOrders };
};
