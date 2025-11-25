# Implementation Plan: E2E No-Authentication Tests

**Feature**: `e2e-no-auth`
**Version**: `1.0.0`
**Status**: `DRAFT`

## 1. Overview

This document outlines the technical plan to implement an end-to-end test suite for the Grid webapp's "no-authentication" mode. The goal is to create a new, independent test flow that runs alongside the existing authentication-focused E2E tests.

The implementation involves creating a new service orchestration script, a new test specification file, and integrating the new test into the project's build system.

## 2. Technical Implementation

### 2.1. New Setup Script

A new shell script, `tests/e2e/setup/start-no-auth-services.sh`, will be created to manage the services for this test scenario. This script will be a lightweight version of the existing `start-services.sh`.

**Key Responsibilities:**
1.  **Start PostgreSQL**: Use `docker compose` to start the `postgres` service.
2.  **Run Migrations**: Execute `gridapi db migrate` to ensure the database schema is up-to-date.
3.  **Start `gridapi` in No-Auth Mode**:
    - Unset any `EXTERNAL_IDP_*` or `OIDC_*` environment variables to force `gridapi` into its default no-auth mode.
    - Launch the `gridapi serve` process in the background.
4.  **Seed Test Data**:
    - After `gridapi` is healthy, execute `gridctl` commands to create a predictable test graph.
    - Example:
        ```bash
        # Create two states
        ./bin/gridctl state create "test-producer-no-auth"
        ./bin/gridctl state create "test-consumer-no-auth"

        # Create a dependency between them
        ./bin/gridctl deps add \
            --from "test-producer-no-auth" \
            --from-output "vpc_id" \
            --to "test-consumer-no-auth"
        ```
5.  **Start Webapp**: Launch the Vite development server for the webapp.
6.  **Cleanup**: A `trap` will be set to ensure all background processes (`gridapi`, `webapp`) are terminated on script exit.

### 2.2. New Playwright Test File

A new test file, `tests/e2e/default-flow.spec.ts`, will contain the test logic.

**Test Scenario:**
1.  **Navigate to Root**: The test will open the webapp at the root URL (`/`).
2.  **Verify No-Auth UI**:
    - Assert that the page does not redirect to a login page.
    - Assert that no "Sign In" button is visible.
    - Assert that the main graph view is rendered.
3.  **Verify Seeded Data**:
    - Assert that two state nodes with the text `test-producer-no-auth` and `test-consumer-no-auth` are visible in the graph.
    - Assert that a connecting edge is rendered between these two nodes.

To support this, the existing auth config is renamed to `playwright.config.auth.ts`, to separate the test projects. Default will use `start-no-auth-services.sh` as its `webServer` command and the original auth config will remain unchanged for the auth flow tests using `start-services.sh`.

### 2.3. Build System Integration

To make the new test easy to run, it will be integrated into the project's build and test tooling.

1.  **`package.json`**: A new npm script will be added to the root `package.json`:
    ```json
    "scripts": {
      // ... existing scripts are renamed to "test:e2e:auth" etc.
      "test:e2e:no-auth": "playwright test --project=no-auth"
    }
    ```

2.  **`Makefile`**: A corresponding target will be added to the root `Makefile` for the original test-e2e -> test-e2e-auth replacing the original.
    ```makefile
    .PHONY: test-e2e
    test-e2e: build js/sdk/lib ## Run E2E tests for no-auth mode
    	@echo "Running No-Auth E2E tests..."
    	@pnpm test:e2e:no-auth
    ```

## 3. Execution Strategy

1.  Create `tests/e2e/setup/start-no-auth-services.sh` and ensure it can start all services correctly.
2.  Create `tests/e2e/no-auth-flow.spec.ts` with the test logic.
3.  Update Playwright configuration to define a new test project for the no-auth flow, linking it to the new setup script.
4.  Add the `test:e2e:no-auth` script to `package.json`.
5.  Add the `test-e2e-no-auth` target to the `Makefile`.
6.  Execute the new Make target to run the tests and validate the entire flow.
