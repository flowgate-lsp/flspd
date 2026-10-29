import { useState, useEffect, useMemo } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI } from '../lib/api';
import { targetNode, type FleetNode } from '../lib/nodes';

// The fleet's nodes and the one the per-node settings target.
export const useNodes = () => {
  const { user, getAuthHeader } = useAuth();
  const [nodes, setNodes] = useState<FleetNode[] | null>(null);
  const [chosen, setChosen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const api = useMemo(() => {
    const proxyUrl = (import.meta as { env?: Record<string, string> }).env?.VITE_PROXY_URL;
    return new DashboardAPI(window.location.origin, getAuthHeader, proxyUrl);
  }, [getAuthHeader]);

  useEffect(() => {
    if (!user) return;
    api.listNodes()
      .then((r) => setNodes(r.nodes ?? []))
      .catch((err) => setError(err instanceof Error ? err.message : String(err)));
  }, [user, api]);

  return { nodes, node: targetNode(nodes, chosen), choose: setChosen, error };
};
