import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

// Force Vite cache reload after fixing Svelte 5 event handlers
export default defineConfig({
  plugins: [sveltekit()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    // Local-only integration: keep the browser on one origin while the Go API
    // runs on its own process. Production uses the deployment reverse proxy.
    proxy: {
      '/v1': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: false
      }
    }
  },
  preview: {
    proxy: {
      '/v1': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: false
      }
    }
  }
});
