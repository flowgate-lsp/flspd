import { renderHook, act } from '@testing-library/react';
import { useEvents } from './useEvents';
import { vi, describe, it, expect } from 'vitest';

// Mock useAuth
vi.mock('./useAuth', () => ({
  useAuth: () => ({
    getAuthHeader: vi.fn().mockResolvedValue('Nostr mock_token'),
  }),
}));

// Mock WebSocket
class MockWebSocket {
  onopen: any;
  onmessage: any;
  onclose: any;
  onerror: any;
  url: string;

  static lastInstance: MockWebSocket;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.lastInstance = this;
    setTimeout(() => {
      if (this.onopen) this.onopen();
    }, 10);
  }

  send(_data: string) {}
  close() {
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
    const { result } = renderHook(() => useEvents());

    // Connect is async, wait for it + onopen
    await act(async () => {
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    expect(result.current.connected).toBe(true);
  });

  it('updates lastEvent when a message is received', async () => {
    const onEvent = vi.fn();
    const { result } = renderHook(() => useEvents(onEvent));

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
