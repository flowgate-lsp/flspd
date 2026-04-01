export interface Env {
  ASSETS: { fetch: (request: Request) => Promise<Response> };
  UPSTREAM_API: string;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    // Determine if the request should be proxied to the backend
    let shouldProxy = false;
    
    // Unconditionally proxy all /api requests
    if (url.pathname.startsWith('/api')) {
      shouldProxy = true;
    } 
    // Proxy /dashboard ONLY for API calls (POST) or WebSockets (Upgrade)
    // This allows GET requests for /dashboard to serve the React UI from ASSETS
    else if (url.pathname.startsWith('/dashboard')) {
      const isUpgrade = request.headers.get('Upgrade')?.toLowerCase() === 'websocket';
      if (request.method === 'POST' || isUpgrade) {
        shouldProxy = true;
      }
    }

    if (shouldProxy) {
      const upstreamUrl = new URL(env.UPSTREAM_API);
      const targetUrl = new URL(url.pathname + url.search, upstreamUrl.origin);

      const newHeaders = new Headers(request.headers);
      
      // Preserve the original frontend Host and Proto for Nostr Auth validation on the backend
      newHeaders.set('X-Original-Host', url.hostname);
      newHeaders.set('X-Original-Proto', url.protocol.replace(':', ''));

      // **CRITICAL FIX**: Traefik relies on the Host header to route the request to the correct container.
      // We must replace the 'flsp.flowgate.to' (Frontend Host) with 'flspapi.flowgate.to' (Backend Host).
      newHeaders.set('Host', upstreamUrl.hostname);
      
      const proxyRequest = new Request(targetUrl.toString(), {
        method: request.method,
        headers: newHeaders,
        body: request.body,
        redirect: 'manual',
      });
      try {
        return await fetch(proxyRequest);
      } catch (e) {
        return new Response(`Gateway Error: ${e instanceof Error ? e.message : 'Unknown error'}`, { status: 502 });
      }
    }

    // Otherwise serve static assets (UI, CSS, JS, etc.)
    return env.ASSETS.fetch(request);
  },
};

