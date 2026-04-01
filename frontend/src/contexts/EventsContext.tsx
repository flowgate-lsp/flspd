import React, { createContext, useContext, useState, useEffect, useCallback, useRef } from 'react';
import { useAuth } from '../hooks/useAuth';

export type DashboardEvent = {
  type: string;
  data: any;
  ts: number;
};

interface EventsContextType {
  connected: boolean;
  lastEvent: DashboardEvent | null;
  subscribe: (callback: (event: DashboardEvent) => void) => () => void;
}

const EventsContext = createContext<EventsContextType | undefined>(undefined);

export const EventsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { user, getAuthHeader } = useAuth();
  const [connected, setConnected] = useState(false);
  const [lastEvent, setLastEvent] = useState<DashboardEvent | null>(null);
  
  const listeners = useRef<Set<(e: DashboardEvent) => void>>(new Set());
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const subscribe = useCallback((callback: (e: DashboardEvent) => void) => {
    listeners.current.add(callback);
    return () => {
      listeners.current.delete(callback);
    };
  }, []);

  const connect = useCallback(async () => {
    if (!user) return;
    if (socketRef.current?.readyState === WebSocket.OPEN || socketRef.current?.readyState === WebSocket.CONNECTING) return;

    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      // Normalize URL for signature (strip trailing slash)
      const baseUrl = `${window.location.protocol}//${window.location.host}/dashboard/ws`;
      
      const authHeader = await getAuthHeader(baseUrl, 'GET');
      if (!authHeader) return;
      
      const token = authHeader.replace('Nostr ', '');
      const wsUrl = `${protocol}//${window.location.host}/dashboard/ws?auth=${encodeURIComponent(token)}`;
      
      console.log(`[EventsContext] Connecting to singleton WebSocket: ${wsUrl}`);
      const socket = new WebSocket(wsUrl);
      socketRef.current = socket;

      socket.onopen = () => {
        console.log('[EventsContext] Singleton WebSocket connected');
        setConnected(true);
      };

      socket.onmessage = (event) => {
        try {
          const data: DashboardEvent = JSON.parse(event.data);
          setLastEvent(data);
          listeners.current.forEach(callback => callback(data));
        } catch (err) {
          console.error('[EventsContext] Failed to parse message', err);
        }
      };

      socket.onclose = () => {
        console.log('[EventsContext] Singleton WebSocket disconnected, retrying in 5s...');
        setConnected(false);
        socketRef.current = null;
        if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = setTimeout(connect, 5000);
      };

      socket.onerror = (err) => {
        console.error('[EventsContext] Singleton WebSocket error', err);
        socket.close();
      };
    } catch (err) {
      console.error('[EventsContext] Connection failed', err);
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = setTimeout(connect, 5000);
    }
  }, [user, getAuthHeader]);

  useEffect(() => {
    if (user) {
      connect();
    } else {
      socketRef.current?.close();
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
    }

    return () => {
      socketRef.current?.close();
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
    };
  }, [user, connect]);

  const value = {
    connected,
    lastEvent,
    subscribe
  };

  return <EventsContext.Provider value={value}>{children}</EventsContext.Provider>;
};

export const useEventsContext = () => {
  const context = useContext(EventsContext);
  if (context === undefined) {
    throw new Error('useEventsContext must be used within an EventsProvider');
  }
  return context;
};
