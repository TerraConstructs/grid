import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for No-Auth E2E tests.
 *
 * These tests validate that the webapp functions correctly when the gridapi
 * server is running with authentication disabled.
 */
export default defineConfig({
  testDir: './tests/e2e',

  /* Only run the default flow tests */
  testMatch: /default-flow\.spec\.ts/,

  /* No global setup needed for this flow */
  globalSetup: undefined,

  /* Test timeout */
  timeout: 60 * 1000,

  /* Expect timeout for assertions */
  expect: {
    timeout: 10 * 1000,
  },

  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,

  reporter: [
    ['html', { outputFolder: 'playwright-report-no-auth' }],
    ['list'],
    ...(process.env.CI ? [['junit', { outputFile: 'test-results/e2e-no-auth-results.xml' }] as const] : []),
  ],

  use: {
    baseURL: process.env.WEBAPP_URL || 'http://localhost:5173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 15 * 1000,
    navigationTimeout: 30 * 1000,
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  /* Web server configuration for the no-auth flow */
  webServer: {
    command: './tests/e2e/setup/start-no-auth-services.sh',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 120 * 1000, // 2 minutes to start all services
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
