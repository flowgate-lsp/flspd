import { useState, useEffect, useCallback } from 'react';
import { useAuth } from './useAuth';

type DashboardEvent = {
  type: string;
  data: any;
  ts: number;
};

export const useEvents = (onEvent?: (event: DashboardEvent) => void) => {
  const { getAuthHeader } = useAuth();
  const [connected, setConnected] = useState(false);
  const [lastEvent, setLastEvent] = useState<DashboardEvent | null>(null);

  const connect = useCallback(async () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const baseUrl = `${window.location.protocol}//${window.location.host}/dashboard/ws`;
    const authHeader = await getAuthHeader(baseUrl, 'GET');
    const token = authHeader.replace('Nostr ', '');
    const wsUrl = `${protocol}//${window.location.host}/dashboard/ws?auth=${encodeURIComponent(token)}`;
    
    console.log(`Connecting to WebSocket: ${wsUrl}`);
    const socket = new WebSocket(wsUrl);

    socket.onopen = () => {
      console.log('Dashboard WebSocket connected');
      setConnected(true);
    };

    socket.onmessage = (event) => {
      try {
        const data: DashboardEvent = JSON.parse(event.data);
        setLastEvent(data);
        if (onEvent) onEvent(data);
      } catch (err) {
        console.error('Failed to parse WebSocket message', err);
      }
    };

    socket.onclose = () => {
      console.log('Dashboard WebSocket disconnected, retrying in 5s...');
      setConnected(false);
      setTimeout(connect, 5000);
    };

    socket.onerror = (err) => {
      console.error('Dashboard WebSocket error', err);
      socket.close();
    };

    return socket;
  }, [getAuthHeader, onEvent]);

  useEffect(() => {
    let socket: WebSocket | undefined;
    connect().then(s => {
      socket = s;
    });
    
    return () => {
      socket?.close();
    };
  }, [connect]);

  return { connected, lastEvent };
};
