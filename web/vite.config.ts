import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const isDev = mode === 'development';
  // VITE_PROXY_URL is exposed to client code too (import.meta.env) --
  // AuthContext/api.ts use it as the URL to sign NIP-98 requests against
  // when there's no reverse proxy in front adding X-Forwarded-Host (e.g.
  // `just frontend-dev-prod <remote-url>`, pointing a local dev server
  // straight at a remote backend). It also doubles as this proxy's target
  // in that scenario.
  //
  // DASHBOARD_PROXY_TARGET is deliberately NOT VITE_-prefixed, so Vite
  // never exposes it to client code. It's what the docker-compose dev stack
  // (fronted by deploy/Caddyfile.dev) sets: Caddy's reverse_proxy adds its
  // own X-Forwarded-Host (the external hostname the browser actually used),
  // which the backend's NIP-98 check already trusts over the raw Host
  // header -- so in that flow the client must sign against
  // window.location.origin (its default, unset here), not this internal
  // target. This var only tells Vite's dev-server proxy where to physically
  // forward /dashboard and /info.
  const proxyTarget = process.env.VITE_PROXY_URL || process.env.DASHBOARD_PROXY_TARGET;
  // Reverse-proxied dev hostnames vary per dev; the deploy/caddy dev service
  // normalizes the Host header to localhost before it reaches Vite, so no
  // per-dev allowlist entry is needed here. Set VITE_ALLOWED_HOSTS if you're
  // bypassing that proxy and connecting some other way.
  const allowedHosts = process.env.VITE_ALLOWED_HOSTS
    ? process.env.VITE_ALLOWED_HOSTS.split(',').map((h) => h.trim())
    : undefined;

  return {
    plugins: [
      react(),
      tailwindcss(),
    ],
    server: isDev ? {
      allowedHosts,
      proxy: proxyTarget ? {
        '/dashboard': {
          target: proxyTarget,
          changeOrigin: true,
          secure: false,
          // /dashboard/ws is a WebSocket upgrade (EventsContext's live event
          // feed) -- http-proxy doesn't forward upgrade requests unless
          // told to, so without this the connection silently never reaches
          // the backend at all (no error, just stays "Disconnected").
          ws: true,
        },
        '/info': {
          target: proxyTarget,
          changeOrigin: true,
          secure: false,
        }
      } : undefined
    } : undefined
  };
});
