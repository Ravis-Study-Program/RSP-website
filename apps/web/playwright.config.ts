import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'on-first-retry',
  },
  webServer: {
    command: 'corepack pnpm build:demo && corepack pnpm preview',
    url: 'http://127.0.0.1:4173',
    reuseExistingServer: !process.env.CI,
    env: { VITE_USE_DEMO_DATA: 'true' },
  },
  projects: [
    { name: 'desktop-chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile-chromium', use: { ...devices['Pixel 7'] } },
    {
      name: 'desktop-firefox-compat',
      grep: /@compat/,
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'desktop-webkit-compat',
      grep: /@compat/,
      use: { ...devices['Desktop Safari'] },
    },
    {
      name: 'mobile-webkit-compat',
      grep: /@compat/,
      use: { ...devices['iPhone 15'] },
    },
  ],
});
