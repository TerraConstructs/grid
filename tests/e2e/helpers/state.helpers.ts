/**
 * State management helpers for E2E tests
 *
 * These helpers manage Terraform state creation and verification through the webapp UI.
 */

import { Page, expect } from '@playwright/test';

export interface StateData {
  logicId: string;
  labels?: Record<string, string>;
  description?: string;
}

/**
 * Navigate to the Create State page
 *
 * @param page Playwright page object
 */
export async function navigateToCreateState(page: Page): Promise<void> {
  // Click "Create State" button or navigate to create page
  // Adjust selector based on actual webapp implementation
  const createButton = page.getByRole('button', { name: /create state/i }).or(
    page.getByRole('link', { name: /create state/i })
  );

  await expect(createButton).toBeVisible({ timeout: 10000 });
  await createButton.click();

  // Wait for form to be visible
  await expect(
    page.getByLabel(/logic.*id/i).or(page.getByPlaceholder(/logic.*id/i))
  ).toBeVisible({ timeout: 10000 });
}

/**
 * Create a new Terraform state
 *
 * Fills out the state creation form and submits it.
 *
 * @param page Playwright page object
 * @param stateData State configuration
 * @returns Promise that resolves when state is created
 */
export async function createState(
  page: Page,
  stateData: StateData
): Promise<void> {
  // Fill logic ID
  const logicIdInput = page.getByLabel(/logic.*id/i).or(
    page.getByPlaceholder(/logic.*id/i)
  );
  await logicIdInput.fill(stateData.logicId);

  // Fill labels if provided
  // CreateStatePage has aria-label="Label key 1", "Label value 1", etc.
  // By default there's one empty label pair already present
  if (stateData.labels) {
    const labelEntries = Object.entries(stateData.labels);

    for (let i = 0; i < labelEntries.length; i++) {
      const [key, value] = labelEntries[i];
      const labelIndex = i + 1; // aria-labels are 1-indexed

      // If not the first label, click "Add Label" button to create new input pair
      if (i > 0) {
        const addLabelButton = page.getByRole('button', { name: /add label/i });
        await addLabelButton.click();
      }

      // Fill key and value using aria-label attributes
      await page.getByLabel(`Label key ${labelIndex}`).fill(key);
      await page.getByLabel(`Label value ${labelIndex}`).fill(value);
    }
  }

  // Fill description if provided (CreateStatePage doesn't have description field yet)
  if (stateData.description) {
    const descInput = page.getByLabel(/description/i);
    if (await descInput.isVisible().catch(() => false)) {
      await descInput.fill(stateData.description);
    }
  }

  // Submit the form by clicking the submit button
  // Use type="submit" selector to distinguish from "Create State" button in header
  const submitButton = page.locator('button[type="submit"]').filter({
    hasText: /create state|submit/i
  });
  await submitButton.click();
}

/**
 * Wait for a state to appear in the state list
 *
 * @param page Playwright page object
 * @param logicId Logic ID of the state to wait for
 * @param timeout Maximum time to wait in milliseconds (default: 10000)
 */
export async function waitForStateInList(
  page: Page,
  logicId: string,
  timeout: number = 10000
): Promise<void> {
  // Navigate to states list if not already there
  const statesLink = page.getByRole('link', { name: /states/i });
  if (await statesLink.isVisible({ timeout: 2000 }).catch(() => false)) {
    await statesLink.click();
  }

  // Wait for the state to appear in the list
  await expect(page.getByText(logicId)).toBeVisible({ timeout });
}

/**
 * Get success notification message
 *
 * Checks for a success toast/notification after an operation.
 *
 * @param page Playwright page object
 * @returns Success message text or null if not found
 */
export async function getSuccessNotification(page: Page): Promise<string | null> {
  try {
    // Look for common success notification patterns
    const notification = page.locator('[role="alert"], .toast, .notification').filter({
      hasText: /success|created|updated/i,
    });

    await notification.waitFor({ state: 'visible', timeout: 5000 });
    return await notification.textContent();
  } catch {
    return null;
  }
}

/**
 * Get error notification message
 *
 * Checks for an error toast/notification after an operation.
 *
 * @param page Playwright page object
 * @returns Error message text or null if not found
 */
export async function getErrorNotification(page: Page): Promise<string | null> {
  try {
    // Look for common error notification patterns
    const notification = page.locator('[role="alert"], .toast, .notification, .error').filter({
      hasText: /error|fail|forbidden|denied|permission/i,
    });

    await notification.waitFor({ state: 'visible', timeout: 5000 });
    return await notification.textContent();
  } catch {
    return null;
  }
}

/**
 * Verify that a state was created successfully
 *
 * Checks for success notification and verifies state appears in list.
 *
 * @param page Playwright page object
 * @param logicId Logic ID of the created state
 */
export async function verifyStateCreated(
  page: Page,
  logicId: string
): Promise<void> {
  // Check for success notification
  const successMsg = await getSuccessNotification(page);
  expect(successMsg).toBeTruthy();

  // Verify state appears in list
  await waitForStateInList(page, logicId);
}

/**
 * Verify that state creation failed with permission error
 *
 * Checks for permission denied error and verifies state does NOT appear in list.
 *
 * @param page Playwright page object
 * @param logicId Logic ID that was attempted
 */
export async function verifyStateCreationForbidden(
  page: Page,
  logicId: string
): Promise<void> {
  // Check for error notification with permission-related text
  const errorMsg = await getErrorNotification(page);
  expect(errorMsg).toBeTruthy();
  expect(errorMsg?.toLowerCase()).toMatch(/forbidden|denied|permission|unauthorized/);

  // Verify state does NOT appear in list
  try {
    await waitForStateInList(page, logicId, 3000);
    throw new Error(`State ${logicId} should not exist but was found in list`);
  } catch (error) {
    // Expected - state should not exist
    if (error instanceof Error && error.message.includes('should not exist')) {
      throw error;
    }
    // Otherwise, timeout is expected
  }
}

/**
 * Create a unique logic ID for testing
 *
 * Generates a logic ID with timestamp to avoid collisions.
 *
 * @param prefix Prefix for the logic ID (default: 'test')
 * @returns Unique logic ID
 */
export function generateTestLogicId(prefix: string = 'test'): string {
  const timestamp = Date.now();
  return `${prefix}-${timestamp}`;
}
