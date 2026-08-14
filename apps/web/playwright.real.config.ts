import { defineConfig, devices } from '@playwright/test';

const baseURL = process.env.RSP_E2E_BASE_URL;
if (!baseURL)
  throw new Error(
    'RSP_E2E_BASE_URL is required for the real-stack Playwright suite.',
  );

const target = new URL(baseURL);
if (!['127.0.0.1', 'localhost', '[::1]', '::1'].includes(target.hostname)) {
  throw new Error(
    'The mutating real-stack suite is restricted to a loopback development stack.',
  );
}
if (
  process.env.APP_ENV !== 'test' ||
  process.env.RSP_E2E_ALLOW_MUTATION !== 'true'
) {
  throw new Error(
    'The mutating real-stack suite requires APP_ENV=test and RSP_E2E_ALLOW_MUTATION=true.',
  );
}
if (!/^rsp-real-e2e(?:-|$)/.test(process.env.COMPOSE_PROJECT_NAME ?? '')) {
  throw new Error(
    'The mutating real-stack suite requires an rsp-real-e2e Compose project.',
  );
}

export default defineConfig({
  testDir: './e2e-real',
  outputDir: './test-results-real',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: process.env.CI
    ? [
        ['github'],
        ['html', { outputFolder: 'playwright-report-real', open: 'never' }],
      ]
    : 'list',
  use: {
    ...devices['Desktop Chrome'],
    baseURL,
    trace: 'retain-on-failure',
  },
});
