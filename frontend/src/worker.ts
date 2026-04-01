export interface Env {
  ASSETS: { fetch: (request: Request) => Promise<Response> };
  UPSTREAM_API: string;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    // Proxy logic for API and Dashboard
    if (url.pathname.startsWith('/api') || url.pathname.startsWith('/dashboard')) {
      const upstreamUrl = new URL(env.UPSTREAM_API);
      const targetUrl = new URL(url.pathname + url.search, upstreamUrl.origin);

      // Clone headers and specifically handle WebSocket upgrade if present
      const newHeaders = new Headers(request.headers);
      
      // Construct the proxy request
      const proxyRequest = new Request(targetUrl.toString(), {
        method: request.method,
        headers: newHeaders,
        body: request.body,
        redirect: 'manual',
      });

      try {
        // Cloudflare's fetch() will automatically handle WebSocket upgrades 
        // if the 'Upgrade: websocket' header is present in proxyRequest.
        return await fetch(proxyRequest);
      } catch (e) {
        return new Response(`Gateway Error: ${e instanceof Error ? e.message : 'Unknown error'}`, { status: 502 });
      }
    }

    // Otherwise serve static assets from the binding
    return env.ASSETS.fetch(request);
  },
};

