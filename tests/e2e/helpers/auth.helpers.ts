/**
 * Authentication helpers for E2E tests
 *
 * These helpers manage the Keycloak OAuth2/OIDC login flow:
 * 1. Click login button on webapp
 * 2. Redirect to Keycloak login page
 * 3. Enter credentials
 * 4. Redirect back to webapp with session
 */

import { Page, expect } from '@playwright/test';

/**
 * Login via Keycloak SSO
 *
 * Navigates through the full OAuth flow:
 * - Webapp → Keycloak login page
 * - Enter credentials
 * - Keycloak → Webapp with session cookie
 *
 * @param page Playwright page object
 * @param email User email (e.g., alice@example.com)
 * @param password User password (default: test123)
 */
export async function loginViaKeycloak(
  page: Page,
  email: string,
  password: string = 'test123'
): Promise<void> {
  // Navigate to webapp
  await page.goto('/');

  // Click "Sign In with SSO" button (external IdP mode)
  // Button text is "Sign In with SSO" from LoginPage.tsx
  const loginButton = page.getByRole('button', { name: /sign in/i });
  await expect(loginButton).toBeVisible({ timeout: 10000 });
  await loginButton.click();

  // Wait for redirect to Keycloak
  await page.waitForURL(/.*localhost:8443.*/, { timeout: 10000 });

  // Fill in Keycloak login form
  await page.getByLabel(/username|email/i).fill(email);
  await page.getByLabel(/password/i).fill(password);

  // Submit the form
  await page.getByRole('button', { name: /sign in|log in/i }).click();

  // Wait for redirect back to webapp
  await page.waitForURL(/.*localhost:5173.*/, { timeout: 10000 });

  // Verify we're logged in by checking for the user profile button (with username)
  // The AuthStatus component shows a button with User icon + username + chevron
  // The Sign Out button is hidden in a dropdown until the profile button is clicked
  await expect(page.getByText(email)).toBeVisible({ timeout: 10000 });
}

/**
 * Logout from the webapp
 *
 * Clicks the logout button and waits for session to be cleared.
 *
 * @param page Playwright page object
 */
export async function logout(page: Page): Promise<void> {
  // First, open the AuthStatus dropdown by clicking the user profile button
  // This button contains the User icon and ChevronDown icon
  const profileButton = page.locator('button').filter({ has: page.locator('svg.lucide-chevron-down') });
  await expect(profileButton).toBeVisible({ timeout: 10000 });
  await profileButton.click();

  // Now the dropdown is open, click "Sign Out" button
  const logoutButton = page.getByRole('button', { name: /sign out/i });
  await expect(logoutButton).toBeVisible({ timeout: 10000 });
  await logoutButton.click();

  // Wait for redirect or UI update indicating logout
  // Should see "Sign In with SSO" button again
  await expect(
    page.getByRole('button', { name: /sign in/i })
  ).toBeVisible({ timeout: 10000 });
}

/**
 * Check if user is currently logged in
 *
 * @param page Playwright page object
 * @returns true if logged in, false otherwise
 */
export async function isLoggedIn(page: Page): Promise<boolean> {
  try {
    // Check for the user profile button (with ChevronDown icon)
    // This is visible when logged in, but Sign Out button is hidden until dropdown opens
    const profileButton = page.locator('button').filter({ has: page.locator('svg.lucide-chevron-down') });
    await profileButton.waitFor({ state: 'visible', timeout: 5000 });
    return true;
  } catch {
    return false;
  }
}

/**
 * Get the session cookie value
 *
 * @param page Playwright page object
 * @param cookieName Name of the session cookie (default: 'grid_session')
 * @returns Cookie value or null if not found
 */
export async function getSessionCookie(
  page: Page,
  cookieName: string = 'grid_session'
): Promise<string | null> {
  const cookies = await page.context().cookies();
  const sessionCookie = cookies.find((c) => c.name === cookieName);
  return sessionCookie?.value || null;
}

/**
 * Wait for Keycloak login redirect
 *
 * Useful when you need to wait for the OAuth redirect to complete.
 *
 * @param page Playwright page object
 * @param timeout Maximum time to wait in milliseconds (default: 10000)
 */
export async function waitForLoginRedirect(
  page: Page,
  timeout: number = 10000
): Promise<void> {
  await page.waitForURL(/.*localhost:8443.*/, { timeout });
}

/**
 * Verify user is authenticated and session persists across page reloads
 *
 * @param page Playwright page object
 * @param email Expected user email
 */
export async function verifySessionPersistence(
  page: Page,
  email: string
): Promise<void> {
  // Verify logged in
  expect(await isLoggedIn(page)).toBe(true);

  // Get session cookie before reload
  const cookieBefore = await getSessionCookie(page);
  expect(cookieBefore).toBeTruthy();

  // Reload page
  await page.reload();

  // Verify still logged in
  expect(await isLoggedIn(page)).toBe(true);

  // Verify session cookie persists
  const cookieAfter = await getSessionCookie(page);
  expect(cookieAfter).toBe(cookieBefore);

  // Verify user info displayed
  await expect(page.getByText(email)).toBeVisible({ timeout: 5000 });
}
