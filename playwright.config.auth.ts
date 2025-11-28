import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for Grid E2E tests.
 *
 * These tests validate the complete user authentication flow through the webapp UI,
 * including OAuth2/OIDC with Keycloak and RBAC authorization.
 *
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: './tests/e2e',

  /* Only run the auth flow tests */
  testMatch: /auth-flow\.spec\.ts/,

  /* Global setup script - starts services, seeds test data */
  globalSetup: './tests/e2e/setup/global-setup.ts',

  /* Test timeout - OAuth flows can be slow */
  timeout: 60 * 1000, // 60 seconds per test

  /* Expect timeout for assertions */
  expect: {
    timeout: 10 * 1000, // 10 seconds
  },

  /* Run tests in files in parallel */
  fullyParallel: true,

  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,

  /* Retry on CI only - auth flows can be flaky */
  retries: process.env.CI ? 2 : 0,

  /* Opt out of parallel tests on CI to avoid race conditions */
  workers: process.env.CI ? 1 : undefined,

  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: [
    ['html', { outputFolder: 'playwright-report' }],
    ['list'],
    // JUnit reporter for CI integration
    ...(process.env.CI ? [['junit', { outputFile: 'test-results/e2e-results.xml' }] as const] : []),
  ],

  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL for the webapp - Vite dev server default */
    baseURL: process.env.WEBAPP_URL || 'http://localhost:5173',

    /* Collect trace on failure for debugging */
    trace: 'retain-on-failure',

    /* Take screenshots on failure */
    screenshot: 'only-on-failure',

    /* Record video on failure */
    video: 'retain-on-failure',

    /* Maximum time each action can take */
    actionTimeout: 15 * 1000, // 15 seconds

    /* Navigation timeout for page loads */
    navigationTimeout: 30 * 1000, // 30 seconds
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },

    // Uncomment to test on additional browsers
    // {
    //   name: 'firefox',
    //   use: { ...devices['Desktop Firefox'] },
    // },

    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },
  ],

  /* Web server configuration - starts all required services */
  webServer: {
    command: './tests/e2e/setup/start-services.sh',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 120 * 1000, // 2 minutes to start all services
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
