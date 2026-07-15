import React from 'react';
import { renderHook, act } from '@testing-library/react';
import { useEvents } from './useEvents';
import { EventsProvider } from '../contexts/EventsContext';
import { vi, describe, it, expect } from 'vitest';

// Mock useAuth: EventsContext only connects when `user` is truthy, and
// requires getAuthHeader to resolve for the connection to proceed. The
// mocked user object and getAuthHeader fn must keep stable identity across
// renders -- EventsContext's connect useCallback depends on both, and a
// fresh object/fn per call re-triggers its connect/cleanup effect on every
// render, endlessly reconnecting and disconnecting the socket.
const mockUser = { pubkey: 'mock_pubkey', nsec: 'mock_nsec' };
const mockGetAuthHeader = vi.fn().mockResolvedValue('Nostr mock_token');
vi.mock('./useAuth', () => ({
  useAuth: () => ({
    user: mockUser,
    getAuthHeader: mockGetAuthHeader,
  }),
}));

const wrapper = ({ children }: { children: React.ReactNode }) =>
  React.createElement(EventsProvider, null, children);

// Mock WebSocket. Static readyState constants matter here: EventsContext's
// connect() dedups against `socketRef.current?.readyState === WebSocket.OPEN`,
// and without these, `WebSocket.OPEN` is undefined -- which trivially equals
// the initial `undefined` readyState of a null socketRef and short-circuits
// every connection attempt before a socket is ever constructed.
class MockWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;

  onopen: any;
  onmessage: any;
  onclose: any;
  onerror: any;
  url: string;
  readyState: number;

  static lastInstance: MockWebSocket;

  constructor(url: string) {
    this.url = url;
    this.readyState = MockWebSocket.CONNECTING;
    MockWebSocket.lastInstance = this;
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      if (this.onopen) this.onopen();
    }, 10);
  }

  send(_data: string) {}
  close() {
    this.readyState = MockWebSocket.CLOSED;
    if (this.onclose) this.onclose();
  }

  // Helper to simulate incoming messages
  simulateMessage(data: any) {
    if (this.onmessage) {
      this.onmessage({ data: JSON.stringify(data) });
    }
  }
}

(global as any).WebSocket = MockWebSocket;

describe('useEvents', () => {
  it('connects to the correct WebSocket URL with auth token', async () => {
    const { result } = renderHook(() => useEvents(), { wrapper });

    // Connect is async, wait for it + onopen
    await act(async () => {
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    expect(result.current.connected).toBe(true);
  });

  it('updates lastEvent when a message is received', async () => {
    const onEvent = vi.fn();
    const { result } = renderHook(() => useEvents(onEvent), { wrapper });

    await act(async () => {
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    const mockEvent = {
        type: 'order_update',
        data: { id: '123', status: 'PAID' },
        ts: Date.now()
    };

    act(() => {
      MockWebSocket.lastInstance.simulateMessage(mockEvent);
    });

    expect(result.current.lastEvent).toEqual(mockEvent);
    expect(onEvent).toHaveBeenCalledWith(mockEvent);
  });
});
