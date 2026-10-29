import { renderHook, act, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { RpcError, METHOD_NOT_FOUND } from '../lib/api';
import { UNNAMED_NODE } from '../lib/nodes';
import { useNodes } from './useNodes';
import { useSettings } from './useSettings';
import { useAnnounceState } from './useAnnounceState';

// T8-6: the node-selector hooks must never show or save one node's data
// while another node is targeted, and must not wait forever on list_nodes.

const mockUser = { pubkey: 'mock_pubkey', nsec: 'mock_nsec' };
const mockGetAuthHeader = vi.fn().mockResolvedValue('Nostr mock_token');
vi.mock('./useAuth', () => ({
  useAuth: () => ({ user: mockUser, getAuthHeader: mockGetAuthHeader }),
}));

// Each API call returns a promise the test settles, keyed by method and node.
type Pending = { resolve: (v: unknown) => void; reject: (e: unknown) => void };
const pending = new Map<string, Pending[]>();
const calls: { method: string; node: string | null | undefined; params?: unknown }[] = [];

function defer(method: string, node?: string | null, params?: unknown) {
  calls.push({ method, node, params });
  return new Promise((resolve, reject) => {
    const key = `${method}:${node ?? ''}`;
    pending.set(key, [...(pending.get(key) ?? []), { resolve, reject }]);
  });
}

function settle(method: string, node: string, outcome: { value?: unknown; error?: unknown }) {
  const key = `${method}:${node}`;
  const queue = pending.get(key) ?? [];
  const next = queue.shift();
  if (!next) throw new Error(`no pending ${key}`);
  if (outcome.error !== undefined) next.reject(outcome.error);
  else next.resolve(outcome.value);
}

vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>();
  class DashboardAPI {
    listNodes() { return defer('list_nodes'); }
    getConfig(node?: string | null) { return defer('get_config', node); }
    updateConfig(config: unknown, node?: string | null) { return defer('update_config', node, config); }
    getAnnounceState(node?: string | null) { return defer('get_announce_state', node); }
    setAnnounceState(action: string, node?: string | null) { return defer('set_announce_state', node, action); }
    recheckAnnouncement(node?: string | null) { return defer('recheck_announcement', node); }
  }
  return { ...actual, DashboardAPI };
});

beforeEach(() => {
  pending.clear();
  calls.length = 0;
});

describe('useNodes (R4-F1)', () => {
  it('targets the unnamed node of a backend without list_nodes', async () => {
    const { result } = renderHook(() => useNodes());
    await act(async () => settle('list_nodes', '', { error: new RpcError(METHOD_NOT_FOUND, 'Method not found') }));
    expect(result.current.node).toBe(UNNAMED_NODE);
    expect(result.current.error).toBeNull();
  });

  it('reports a failed list instead of waiting, and retries', async () => {
    const { result } = renderHook(() => useNodes());
    await act(async () => settle('list_nodes', '', { error: new Error('HTTP error! status: 502') }));
    expect(result.current.node).toBeNull();
    expect(result.current.error).toBe('HTTP error! status: 502');

    act(() => result.current.retry());
    expect(result.current.error).toBeNull();
    await act(async () => settle('list_nodes', '', { value: { nodes: [{ node_id: '02aa', name: 'A' }] } }));
    expect(result.current.node).toBe('02aa');
  });

  it('shows an empty fleet as such', async () => {
    const { result } = renderHook(() => useNodes());
    await act(async () => settle('list_nodes', '', { value: { nodes: [] } }));
    expect(result.current.nodes).toEqual([]);
    expect(result.current.node).toBeNull();
  });
});

describe('useSettings (R4-F3, R4-F5)', () => {
  it('drops a late answer for the node no longer targeted', async () => {
    const { result, rerender } = renderHook(({ node }) => useSettings(node), { initialProps: { node: '02aa' as string | null } });
    rerender({ node: '03bb' });
    await act(async () => settle('get_config', '03bb', { value: { config: { name: 'B' }, announce_relays: [] } }));
    await act(async () => settle('get_config', '02aa', { value: { config: { name: 'A' }, announce_relays: [] } }));
    expect(result.current.config).toEqual({ name: 'B' });
  });

  it('clears the config when the node changes', async () => {
    const { result, rerender } = renderHook(({ node }) => useSettings(node), { initialProps: { node: '02aa' as string | null } });
    await act(async () => settle('get_config', '02aa', { value: { config: { name: 'A' }, announce_relays: [] } }));
    expect(result.current.config).toEqual({ name: 'A' });
    rerender({ node: '03bb' });
    expect(result.current.config).toBeNull();
  });

  it('refuses to save before the targeted node loaded', async () => {
    const { result, rerender } = renderHook(({ node }) => useSettings(node), { initialProps: { node: '02aa' as string | null } });
    await act(async () => settle('get_config', '02aa', { value: { config: { name: 'A' }, announce_relays: [] } }));
    rerender({ node: '03bb' });
    await act(async () => settle('get_config', '03bb', { error: new Error('boom') }));
    let saved = true;
    await act(async () => { saved = await result.current.updateConfig({ name: 'A' }); });
    expect(saved).toBe(false);
    expect(calls.some((c) => c.method === 'update_config')).toBe(false);
  });

  it('calls without a node param against a backend without list_nodes', async () => {
    renderHook(() => useSettings(UNNAMED_NODE));
    await waitFor(() => expect(calls).toContainEqual({ method: 'get_config', node: UNNAMED_NODE, params: undefined }));
  });
});

describe('useAnnounceState (R4-F4)', () => {
  it('never shows the previous node status', async () => {
    const { result, rerender } = renderHook(({ node }) => useAnnounceState(node), { initialProps: { node: '02aa' as string | null } });
    await act(async () => settle('get_announce_state', '02aa', { value: { status: 'open' } }));
    expect(result.current.state).toEqual({ status: 'open' });

    rerender({ node: '03bb' });
    expect(result.current.state).toBeNull();
    await act(async () => settle('get_announce_state', '03bb', { value: { status: 'closed' } }));
    expect(result.current.state).toEqual({ status: 'closed' });
  });

  it('drops a late answer for the node no longer targeted', async () => {
    const { result, rerender } = renderHook(({ node }) => useAnnounceState(node), { initialProps: { node: '02aa' as string | null } });
    rerender({ node: '03bb' });
    await act(async () => settle('get_announce_state', '02aa', { value: { status: 'paused' } }));
    expect(result.current.state).toBeNull();
    await act(async () => settle('get_announce_state', '03bb', { value: { status: 'open' } }));
    expect(result.current.state).toEqual({ status: 'open' });
  });
});
