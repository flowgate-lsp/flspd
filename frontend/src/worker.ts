export interface Env {
  ASSETS: { fetch: (request: Request) => Promise<Response> };
  UPSTREAM_API: string;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    // 1. Proxy API requests to the backend server
    if (url.pathname.startsWith('/api')) {
      const upstreamUrl = new URL(env.UPSTREAM_API);
      
      // Construct the new target URL by appending the remainder of the path
      // Note: We strip '/api' if UPSTREAM_API already includes it or handle it based on backend config
      // Current Traefik expects /api, but strips it. Worker will forward /api/info -> flsp.flowgate.to/api/info
      const targetUrl = new URL(url.pathname + url.search, upstreamUrl.origin);

      const proxyRequest = new Request(targetUrl.toString(), {
        method: request.method,
        headers: request.headers,
        body: request.body,
        redirect: 'manual',
      });

      try {
        return await fetch(proxyRequest);
      } catch (e) {
        return new Response(`Gateway Error: ${e instanceof Error ? e.message : 'Unknown error'}`, { status: 502 });
      }
    }

    // 2. Proxy Dashboard API requests to the dashboard backend server
    if (url.pathname.startsWith('/dashboard')) {
      const upstreamUrl = new URL(env.UPSTREAM_API);
      const targetUrl = new URL(url.pathname + url.search, upstreamUrl.origin);

      const proxyRequest = new Request(targetUrl.toString(), {
        method: request.method,
        headers: request.headers,
        body: request.body,
        redirect: 'manual',
      });

      try {
        return await fetch(proxyRequest);
      } catch (e) {
        return new Response(`Gateway Error: ${e instanceof Error ? e.message : 'Unknown error'}`, { status: 502 });
      }
    }

    // 3. Otherwise serve static assets from the binding
    return env.ASSETS.fetch(request);
  },
};

