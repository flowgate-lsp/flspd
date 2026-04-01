import { useEffect } from 'react';
import { useEventsContext } from '../contexts/EventsContext';
import type { DashboardEvent } from '../contexts/EventsContext';

export const useEvents = (onEvent?: (event: DashboardEvent) => void) => {
  const { connected, lastEvent, subscribe } = useEventsContext();

  useEffect(() => {
    if (!onEvent) return;
    
    // Subscribe to the singleton broadcaster
    const unsubscribe = subscribe(onEvent);
    return () => unsubscribe();
  }, [onEvent, subscribe]);

  return { connected, lastEvent };
};
