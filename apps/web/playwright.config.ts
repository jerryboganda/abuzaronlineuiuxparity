import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  // The local API/web stack is supervised as a shared process. Serializing
  // the browser contexts keeps mocked API routes deterministic and prevents
  // worker contention from being reported as a product regression.
  workers: 1,
  // A fresh Chromium context can be interrupted by the supervised local
  // process lifecycle. Retry only at the Playwright boundary; a product
  // assertion still fails after all attempts.
  retries: 2,
  use: {
    baseURL: 'http://127.0.0.1:5173',
    trace: 'retain-on-failure'
  },
  webServer: {
    command: 'pnpm dev -- --host 127.0.0.1',
    url: 'http://127.0.0.1:5173/login',
    reuseExistingServer: true
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } }
  ]
});
