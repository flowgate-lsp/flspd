import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const isDev = mode === 'development';
  const proxyUrl = process.env.VITE_PROXY_URL;

  return {
    plugins: [
      react(),
      tailwindcss(),
    ],
    server: isDev && proxyUrl ? {
      proxy: {
        '/dashboard': {
          target: proxyUrl,
          changeOrigin: true,
          secure: false,
        }
      }
    } : undefined
  };
});
