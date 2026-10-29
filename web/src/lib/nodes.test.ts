import { describe, expect, it } from 'vitest';
import { targetNode, withNode } from './nodes';

const a = { node_id: '02aa', name: 'A' };
const b = { node_id: '03bb', name: 'B' };

describe('targetNode', () => {
  it('uses the only node without a choice', () => {
    expect(targetNode([a], null)).toBe('02aa');
  });
  it('needs a choice in a fleet', () => {
    expect(targetNode([a, b], null)).toBeNull();
    expect(targetNode([a, b], '03bb')).toBe('03bb');
  });
  it('ignores a choice not in the fleet', () => {
    expect(targetNode([a, b], '02ff')).toBeNull();
  });
  it('waits for the node list', () => {
    expect(targetNode(null, '02aa')).toBeNull();
  });
});

describe('withNode', () => {
  it('adds node only when set', () => {
    expect(withNode({ action: 'pause' }, '03bb')).toEqual({ action: 'pause', node: '03bb' });
    expect(withNode({ action: 'pause' }, null)).toEqual({ action: 'pause' });
  });
});
