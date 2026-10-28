import { describe, it, expect } from 'vitest';
import { toAnnounceForm, fromAnnounceForm, announceHints, charCount, availableActions, relaySummary } from './announce';

describe('announce form', () => {
  it('round-trips a full block, keeping addrs exactly and in order', () => {
    const block = {
      enabled: true,
      title: "Alice's Liquidity",
      content: 'Inbound channels.\nOnion preferred.',
      website: 'https://lsp.example.com',
      image: 'https://lsp.example.com/logo.png',
      operator: { pubkey: 'c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5', relay: 'wss://relay.example.com' },
      languages: ['en', 'fr'],
      addrs: ['[2001:db8::1]:9735', '203.0.113.7:9735'],
    };
    expect(fromAnnounceForm(toAnnounceForm(block))).toEqual(block);
  });

  it('drops blank address lines but never edits a typed address', () => {
    const f = toAnnounceForm({ enabled: true });
    f.addrs = '203.0.113.7:9735\n\n 10.0.0.1:9735';
    expect(fromAnnounceForm(f)?.addrs).toEqual(['203.0.113.7:9735', ' 10.0.0.1:9735']);
  });

  it('keeps no block for an untouched, disabled form', () => {
    expect(fromAnnounceForm(toAnnounceForm(undefined))).toBeUndefined();
  });

  it('keeps a disabled draft', () => {
    const f = toAnnounceForm(undefined);
    f.title = 'Draft';
    expect(fromAnnounceForm(f)).toEqual({ enabled: false, title: 'Draft' });
  });

  it('counts characters as code points', () => {
    expect(charCount('é😀')).toBe(2);
  });

  it('hints at the common mistakes', () => {
    const f = toAnnounceForm({ enabled: true });
    f.title = 'x'.repeat(41);
    f.website = 'http://lsp.example.com';
    f.operatorRelay = 'wss://relay.example.com';
    const hints = announceHints(f);
    expect(hints).toHaveLength(4);
    expect(hints.join(' ')).toContain('at least one address');
  });

  it('asks for a relay with an operator pubkey', () => {
    const f = toAnnounceForm({ enabled: false });
    f.operatorPubkey = 'c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5';
    expect(announceHints(f)).toEqual(['An operator pubkey needs a relay (wss://).']);
  });
});

describe('announcement status actions', () => {
  it('mirrors the server transitions', () => {
    expect(availableActions('open')).toEqual(['pause', 'unlist']);
    expect(availableActions('paused')).toEqual(['resume', 'unlist']);
    expect(availableActions('closed')).toEqual(['resume']);
  });
});

describe('publisher summary', () => {
  it('counts accepting relays', () => {
    expect(relaySummary(null)).toBeNull();
    expect(relaySummary({ last_event: null, self_check: null, blocked: '', last_error: '' })).toBeNull();
    expect(relaySummary({
      last_event: { id: 'ab', created_at: '', expiration: '', status: 'open',
        relays: [{ relay: 'wss://a', accepted: true }, { relay: 'wss://b', accepted: false, error: 'blocked' }] },
      self_check: null, blocked: '', last_error: '',
    })).toBe('1/2 relays accepted');
  });
});
