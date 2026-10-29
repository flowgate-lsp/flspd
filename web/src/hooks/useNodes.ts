import { useState, useEffect, useMemo } from 'react';
import { useAuth } from './useAuth';
import { DashboardAPI, METHOD_NOT_FOUND, RpcError } from '../lib/api';
import { targetNode, UNNAMED_NODE, type FleetNode } from '../lib/nodes';

// The fleet's nodes and the one the per-node settings target. node is null
// while there is nothing to target yet: the list is loading, failed (error),
// is empty, or a fleet waits for a choice. A backend without list_nodes
// targets UNNAMED_NODE, its only node (T8-6, R4-F1).
export const useNodes = () => {
  const { user, getAuthHeader } = useAuth();
  const [nodes, setNodes] = useState<FleetNode[] | null>(null);
  const [unnamed, setUnnamed] = useState(false);
  const [chosen, setChosen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  const api = useMemo(() => {
    const proxyUrl = (import.meta as { env?: Record<string, string> }).env?.VITE_PROXY_URL;
    return new DashboardAPI(window.location.origin, getAuthHeader, proxyUrl);
  }, [getAuthHeader]);

  useEffect(() => {
    if (!user) return;
    let live = true;
    api.listNodes()
      .then((r) => {
        if (!live) return;
        setUnnamed(false);
        setNodes(r.nodes ?? []);
      })
      .catch((err) => {
        if (!live) return;
        if (err instanceof RpcError && err.code === METHOD_NOT_FOUND) {
          setUnnamed(true);
          return;
        }
        setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      live = false;
    };
  }, [user, api, attempt]);

  return {
    nodes,
    node: unnamed ? UNNAMED_NODE : targetNode(nodes, chosen),
    choose: setChosen,
    error,
    retry: () => {
      setError(null);
      setAttempt((n) => n + 1);
    },
  };
};
