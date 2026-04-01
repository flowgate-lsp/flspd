import '@testing-library/jest-dom'
import { vi } from 'vitest'

// Mock Nostr window object
const mockNostr = {
  getPublicKey: vi.fn(),
  signEvent: vi.fn(),
};

Object.defineProperty(window, 'nostr', {
  value: mockNostr,
  writable: true,
});

// Mock ResizeObserver
class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

window.ResizeObserver = ResizeObserver;
