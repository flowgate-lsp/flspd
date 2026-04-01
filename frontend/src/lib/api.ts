export class DashboardAPI {
  private baseUrl: string;
  private getAuthHeader: (url: string, method: string) => Promise<string>;

  constructor(baseUrl: string, getAuthHeader: (url: string, method: string) => Promise<string>) {
    this.baseUrl = baseUrl;
    this.getAuthHeader = getAuthHeader;
  }

  async request(method: string, params: any = {}) {
    const url = `${this.baseUrl}/dashboard`;
    const authHeader = await this.getAuthHeader(url, 'POST');

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authHeader,
      },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method,
        params,
        id: Math.random().toString(36).substring(7),
      }),
    });

    if (!response.ok) {
      if (response.status === 401 || response.status === 403) {
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

  getConfig() {
    return this.request('get_config');
  }

  updateConfig(config: any) {
    return this.request('update_config', { config });
  }

  getOrderDetail(orderId: string) {
    return this.request('get_order_detail', { order_id: orderId });
  }

  getRadarData() {
    return this.request('get_radar_data');
  }
}
