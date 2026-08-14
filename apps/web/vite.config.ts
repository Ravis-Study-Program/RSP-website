import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': new URL('./src', import.meta.url).pathname } },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api/auth': {
        target: process.env.VITE_AUTH_PROXY_TARGET ?? 'http://auth:3001',
        changeOrigin: true,
      },
      '/api/v2': {
        target: process.env.VITE_API_PROXY_TARGET ?? 'http://api:8080',
        changeOrigin: true,
      },
    },
  },
  preview: { port: 4173, strictPort: true },
  build: {
    // Keep production artifacts free of source-only fixtures and implementation
    // details. Development still has Vite's native source maps.
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': [
            'react',
            'react-dom',
            'react-router-dom',
            '@tanstack/react-query',
          ],
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: true,
    globals: true,
    exclude: ['e2e/**', 'e2e-real/**', 'node_modules/**', 'dist/**'],
  },
});
