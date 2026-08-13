import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': new URL('./src', import.meta.url).pathname } },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api/auth': { target: 'http://auth:3001', changeOrigin: true },
      '/api/v2': { target: 'http://api:8080', changeOrigin: true },
    },
  },
  preview: { port: 4173, strictPort: true },
  build: { sourcemap: true },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: true,
    globals: true,
    exclude: ['e2e/**', 'node_modules/**', 'dist/**'],
  },
});
