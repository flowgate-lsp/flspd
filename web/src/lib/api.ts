import { withNode, type FleetNode } from './nodes';

export class DashboardAPI {
  private baseUrl: string;
  private authUrl: string;
  private getAuthHeader: (url: string, method: string, body?: string) => Promise<string>;

  constructor(baseUrl: string, getAuthHeader: (url: string, method: string, body?: string) => Promise<string>, authUrl?: string) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.authUrl = (authUrl || baseUrl).replace(/\/$/, '');
    this.getAuthHeader = getAuthHeader;
  }

  async request(method: string, params: any = {}) {
    const url = `${this.baseUrl}/dashboard`;
    const authUrl = `${this.authUrl}/dashboard`;
    const body = JSON.stringify({
      jsonrpc: '2.0',
      method,
      params,
      id: Math.random().toString(36).substring(7),
    });
    const authHeader = await this.getAuthHeader(authUrl, 'POST', body);

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authHeader,
      },
      body,
    });

    if (!response.ok) {
      if (response.status === 401 || response.status === 403) {
        // Dispatch global event for useAuth to handle logout
        window.dispatchEvent(new CustomEvent('flspd_unauthorized'));
        throw new Error('Unauthorized');
      }
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    if (data.error) {
      throw new Error(data.error.message);
    }

    return data.result;
  }

  async listPeers() {
    return this.request('list_peers');
  }

  async listJitChannels() {
    return this.request('list_jit_channels');
  }

  getSummary() {
    return this.request('get_summary');
  }

  listOrders(limit = 50, offset = 0) {
    return this.request('list_orders', { limit, offset });
  }

  // The fleet's nodes, for the node selector.
  listNodes(): Promise<{ nodes: FleetNode[] }> {
    return this.request('list_nodes');
  }

  // The per-node methods take the selected node; the server requires it
  // when the fleet has more than one node.
  getConfig(node?: string | null) {
    return this.request('get_config', withNode({}, node));
  }

  updateConfig(config: any, node?: string | null) {
    return this.request('update_config', withNode({ config }, node));
  }

  // NIP-LSP announcement status (T8-2): open, paused or closed.
  getAnnounceState(node?: string | null) {
    return this.request('get_announce_state', withNode({}, node));
  }

  setAnnounceState(action: 'pause' | 'resume' | 'unlist', node?: string | null) {
    return this.request('set_announce_state', withNode({ action }, node));
  }

  // Lifts a failed self-check's block once its cause is fixed (T8-4).
  recheckAnnouncement(node?: string | null) {
    return this.request('recheck_announcement', withNode({}, node));
  }

  getOrderDetail(orderId: string) {
    return this.request('get_order_detail', { order_id: orderId });
  }

  getRadarData() {
    return this.request('get_radar_data');
  }

  getAnalytics() {
    return this.request('get_analytics');
  }
}
