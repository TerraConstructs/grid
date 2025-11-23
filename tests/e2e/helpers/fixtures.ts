/**
 * Playwright test fixtures for Grid E2E tests
 *
 * Provides reusable test fixtures including authenticated browser contexts
 * for different user roles.
 */

import { test as base, Page } from '@playwright/test';
import { loginViaKeycloak } from './auth.helpers';

interface GridFixtures {
  /**
   * Page authenticated as alice@example.com (product-engineer role)
   * Has access to states with env=dev label only
   */
  authenticatedPage: Page;

  /**
   * Page authenticated as platform@example.com (platform-engineer role)
   * Has full admin access (wildcard permissions)
   */
  platformEngineerPage: Page;

  /**
   * Page authenticated as newuser@example.com (no groups)
   * Tests JIT provisioning flow
   */
  newUserPage: Page;
}

/**
 * Extended Playwright test with Grid-specific fixtures
 *
 * Usage:
 * ```ts
 * import { test, expect } from './helpers/fixtures';
 *
 * test('product engineer can create dev states', async ({ authenticatedPage }) => {
 *   // authenticatedPage is already logged in as alice@example.com
 *   await authenticatedPage.goto('/states/create');
 *   // ...
 * });
 * ```
 */
export const test = base.extend<GridFixtures>({
  /**
   * Authenticated page as product-engineer (alice@example.com)
   */
  authenticatedPage: async ({ page }, use) => {
    await loginViaKeycloak(page, 'alice@example.com', 'test123');
    await use(page);
  },

  /**
   * Authenticated page as platform-engineer (platform@example.com)
   */
  platformEngineerPage: async ({ page }, use) => {
    await loginViaKeycloak(page, 'platform@example.com', 'test123');
    await use(page);
  },

  /**
   * Authenticated page as new user (newuser@example.com)
   */
  newUserPage: async ({ page }, use) => {
    await loginViaKeycloak(page, 'newuser@example.com', 'test123');
    await use(page);
  },
});

export { expect } from '@playwright/test';
