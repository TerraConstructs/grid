/**
 * E2E Tests for No-Authentication Flow
 *
 * These tests validate that the Grid webapp functions correctly when the
 * gridapi server is running with authentication disabled.
 */

import { test, expect } from '@playwright/test';

test.describe('No-Auth Flows', () => {
  /**
   * Scenario: Webapp loads and displays seeded data in no-auth mode
   *
   * 1. Navigate to the webapp root
   * 2. Verify no login button or authentication flow is triggered
   * 3. Verify the main graph view is visible
   * 4. Verify the states and dependency seeded by `start-no-auth-services.sh` are displayed
   */
  test('should load the graph and display seeded data', async ({ page }) => {
    // The webServer defined in the 'no-auth' project in playwright.config.ts
    // has already started all services and seeded the data.

    // Navigate to the webapp root
    await page.goto('/');

    // 1. Verify no login elements are visible
    // The app should directly render the main view without any auth UI
    await expect(page.getByRole('button', { name: /sign in/i })).not.toBeVisible();
    await expect(page.getByRole('heading', { name: /sign in/i })).not.toBeVisible();

    // 2. Verify the main graph view is visible by checking for ReactFlow's viewport
    // ReactFlow renders nodes as DOM elements with .react-flow__viewport class
    const graphViewport = page.locator('.react-flow__viewport');
    await expect(graphViewport).toBeVisible({ timeout: 15000 }); // Increase timeout for initial render

    // 3. Verify the seeded states are visible on the graph using data-testid
    // Use the new data-testid selectors added for reliable testing
    const producerNode = page.locator('[data-testid="graph-node-test-producer-no-auth"]');
    const consumerNode = page.locator('[data-testid="graph-node-test-consumer-no-auth"]');

    await expect(producerNode).toBeVisible({ timeout: 10000 });
    await expect(consumerNode).toBeVisible({ timeout: 10000 });

    // 4. Verify a dependency edge exists using data-testid
    // The edge ID format is: {from_guid}_{to_guid}_{from_output}
    // Since we don't know the exact GUIDs at test time, we can check for any edges
    // Exclude hover paths by matching only those WITHOUT "-hover-" in the ID
    const edges = page.locator('[data-testid^="graph-edge-"]:not([data-testid*="-hover-"])');
    await expect(edges).toHaveCount(1, { timeout: 5000 }); // Expect exactly 1 edge from seed data

    // Verify we have at least 2 nodes rendered
    const nodeCount = await page.locator('.react-flow__node').count();
    expect(nodeCount).toBeGreaterThanOrEqual(2);
  });
});
