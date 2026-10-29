// The fleet's node selector for the per-node dashboard methods
// (get_config, update_config, the announcement actions).

export interface FleetNode {
  node_id: string;
  name: string;
}

/** The node the per-node calls target: the chosen one, or the only node. */
export function targetNode(nodes: FleetNode[] | null, chosen: string | null): string | null {
  if (!nodes) return null;
  if (chosen && nodes.some((n) => n.node_id === chosen)) return chosen;
  return nodes.length === 1 ? nodes[0].node_id : null;
}

/** Adds the node param when one is targeted. */
export function withNode<T extends object>(params: T, node?: string | null): T & { node?: string } {
  return node ? { ...params, node } : params;
}
