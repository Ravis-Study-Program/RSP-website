import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    coverage: {
      reporter: ['text', 'json-summary'],
    },
    environment: 'node',
    // Live cases perform several real password hashes and network round trips.
    testTimeout: process.env.AUTH_LIVE_TEST === 'true' ? 30_000 : 5_000,
    include: ['test/**/*.test.ts'],
    restoreMocks: true,
  },
});
