/**
 * E2E Tests for Web Authentication Flows
 *
 * These tests validate the complete authentication and authorization flow
 * through the Grid webapp UI, including:
 * - OAuth2/OIDC with Keycloak
 * - Session management
 * - Group-based RBAC
 * - Just-In-Time (JIT) user provisioning
 *
 * Test Data:
 * - alice@example.com: member of product-engineers (env=dev access only)
 * - platform@example.com: member of platform-engineers (full access)
 * - newuser@example.com: no groups (for JIT provisioning test)
 *
 * All users have password: test123
 */

import { test, expect } from './helpers/fixtures';
import {
  loginViaKeycloak,
  logout,
  verifySessionPersistence,
} from './helpers/auth.helpers';
import {
  createState,
  navigateToCreateState,
  verifyStateCreated,
  verifyStateCreationForbidden,
  generateTestLogicId,
} from './helpers/state.helpers';

test.describe('Authentication Flows', () => {
  /**
   * Scenario 1: Successful Login and Session Persistence
   *
   * Tests the complete OAuth2/OIDC login flow with Keycloak:
   * 1. User clicks login button
   * 2. Redirected to Keycloak
   * 3. Enters credentials
   * 4. Redirected back to webapp with session
   * 5. Session persists across page reloads
   */
  test('successful login and session persistence', async ({ page }) => {
    // Navigate to webapp
    await page.goto('/');

    // Perform login via Keycloak
    await loginViaKeycloak(page, 'alice@example.com', 'test123');

    // Verify session persists across page reload
    await verifySessionPersistence(page, 'alice@example.com');

    // Logout
    await logout(page);

    // Verify logged out - should see login button again
    await expect(page.getByRole('button', { name: /login/i })).toBeVisible();
  });

  /**
   * Scenario 2: Group-Based Permission - Allowed Action
   *
   * Tests RBAC enforcement for allowed operations:
   * - alice@example.com is in product-engineers group
   * - product-engineer role has access to states with env=dev
   * - Creating a state with env=dev should succeed
   */
  test('group-based permission - allowed action (env=dev)', async ({
    authenticatedPage,
  }) => {
    // authenticatedPage is already logged in as alice@example.com

    // Navigate to Create State page
    await navigateToCreateState(authenticatedPage);

    // Create state with env=dev label (allowed for product-engineers)
    const logicId = generateTestLogicId('dev-state');
    await createState(authenticatedPage, {
      logicId,
      labels: {
        env: 'dev',
      },
      description: 'Test development state',
    });

    // Verify state was created successfully
    await verifyStateCreated(authenticatedPage, logicId);
  });

  /**
   * Scenario 3: Group-Based Permission - Forbidden Action
   *
   * Tests RBAC enforcement for forbidden operations:
   * - alice@example.com is in product-engineers group
   * - product-engineer role does NOT have access to states with env=prod
   * - Creating a state with env=prod should fail with permission error
   */
  test('group-based permission - forbidden action (env=prod)', async ({
    authenticatedPage,
  }) => {
    // authenticatedPage is already logged in as alice@example.com

    // Navigate to Create State page
    await navigateToCreateState(authenticatedPage);

    // Attempt to create state with env=prod label (forbidden for product-engineers)
    const logicId = generateTestLogicId('prod-state');
    await createState(authenticatedPage, {
      logicId,
      labels: {
        env: 'prod',
      },
      description: 'Test production state',
    });

    // Verify state creation failed with permission error
    await verifyStateCreationForbidden(authenticatedPage, logicId);
  });

  /**
   * Scenario 4: First-Time Login JIT Provisioning
   *
   * Tests Just-In-Time user provisioning:
   * - newuser@example.com exists in Keycloak but not in Grid database
   * - On first login, Grid creates user record automatically
   * - User can successfully authenticate and access the webapp
   */
  test('first-time login JIT provisioning', async ({ page }) => {
    // Login as new user (exists in Keycloak, not in Grid DB)
    await loginViaKeycloak(page, 'newuser@example.com', 'test123');

    // Verify successful login (proves JIT provisioning worked)
    await expect(page.getByText('newuser@example.com')).toBeVisible({
      timeout: 10000,
    });

    // Verify user can access basic features
    // Note: newuser has no groups, so may have limited permissions
    await expect(
      page.getByRole('button', { name: /logout/i })
    ).toBeVisible();

    // Logout
    await logout(page);
  });
});

test.describe('Authorization Flows - Platform Engineer', () => {
  /**
   * Platform engineers have full admin access (wildcard permissions)
   * They should be able to create states with any labels
   */
  test('platform engineer can create prod states', async ({
    platformEngineerPage,
  }) => {
    // platformEngineerPage is already logged in as platform@example.com

    // Navigate to Create State page
    await navigateToCreateState(platformEngineerPage);

    // Create state with env=prod label (allowed for platform-engineers)
    const logicId = generateTestLogicId('platform-prod-state');
    await createState(platformEngineerPage, {
      logicId,
      labels: {
        env: 'prod',
      },
      description: 'Platform engineer production state',
    });

    // Verify state was created successfully
    await verifyStateCreated(platformEngineerPage, logicId);
  });
});
