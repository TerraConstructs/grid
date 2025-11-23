/**
 * Playwright Global Setup
 *
 * This runs once before all tests. The webServer config in playwright.config.ts
 * handles starting services, so this is mainly for validation and environment setup.
 */

import { chromium, FullConfig } from '@playwright/test';

async function globalSetup(config: FullConfig) {
  console.log('[E2E Global Setup] Starting...');

  // Verify environment variables are set (if needed)
  const baseURL = config.use?.baseURL || 'http://localhost:5173';
  console.log(`[E2E Global Setup] Base URL: ${baseURL}`);

  // Optional: Warm up the browser and verify services are accessible
  const browser = await chromium.launch();
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    // Check that webapp is accessible
    console.log('[E2E Global Setup] Checking webapp accessibility...');
    const response = await page.goto(baseURL, {
      timeout: 30000,
      waitUntil: 'domcontentloaded',
    });

    if (!response || !response.ok()) {
      throw new Error(
        `Webapp not accessible at ${baseURL}: ${response?.status()}`
      );
    }

    console.log('[E2E Global Setup] Webapp is accessible');

    // Check that gridapi is accessible
    console.log('[E2E Global Setup] Checking gridapi accessibility...');
    const gridapiUrl = 'http://localhost:8080/health';
    const gridapiResponse = await page.goto(gridapiUrl, {
      timeout: 30000,
    });

    if (!gridapiResponse || !gridapiResponse.ok()) {
      throw new Error(
        `gridapi not accessible at ${gridapiUrl}: ${gridapiResponse?.status()}`
      );
    }

    console.log('[E2E Global Setup] gridapi is accessible');

    // Check that Keycloak is accessible
    console.log('[E2E Global Setup] Checking Keycloak accessibility...');
    const keycloakUrl = 'http://localhost:8443/realms/grid';
    const keycloakResponse = await page.goto(keycloakUrl, {
      timeout: 30000,
    });

    if (!keycloakResponse || !keycloakResponse.ok()) {
      throw new Error(
        `Keycloak not accessible at ${keycloakUrl}: ${keycloakResponse?.status()}`
      );
    }

    console.log('[E2E Global Setup] Keycloak is accessible');
    console.log('[E2E Global Setup] All services verified successfully!');
  } catch (error) {
    console.error('[E2E Global Setup] Service verification failed:', error);
    throw error;
  } finally {
    await context.close();
    await browser.close();
  }

  console.log('[E2E Global Setup] Complete');
}

export default globalSetup;
