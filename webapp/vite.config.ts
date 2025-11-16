import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  optimizeDeps: {
    exclude: ['lucide-react'],
  },
  server: {
    proxy: {
      // Proxy all Grid API requests to gridapi server
      // This is REQUIRED for development to ensure same-origin for session cookies
      // DO NOT REMOVE: httpOnly session cookies won't work across different origins
      '/auth': 'http://localhost:8080',
      '/api': 'http://localhost:8080',
      '/state.v1.StateService': 'http://localhost:8080',
      '/tfstate': 'http://localhost:8080',
    },
  },
});
