# Quickstart: Running No-Auth E2E Tests

**Feature**: `e2e-no-auth`
**Version**: `1.0.0`
**Status**: `DRAFT`

## 1. Overview

This document provides instructions on how to run the new end-to-end (E2E) test suite that validates the Grid webapp in a "no-authentication" mode.

These tests are self-contained and manage their own services, including a database, the `gridapi` server, and the webapp development server.

## 2. Prerequisites

- Node.js 20+
- pnpm 10+
- Docker
- Go 1.22+

## 3. Running the Tests

The simplest way to run the new test suite is by using the dedicated `make` target.

### Standard Headless Run

This command will build the necessary binaries, start all services, run the Playwright tests in headless mode, and tear down the services upon completion.

```bash
make test-e2e-no-auth
```

### Expected Output

You will see logs from the setup script as it starts services, followed by the output from Playwright as it executes the tests. A successful run will look similar to this:

```
$ make test-e2e-no-auth
Running No-Auth E2E tests...
[E2E No-Auth Setup] Starting docker compose services (postgres)...
[E2E No-Auth Setup] Waiting for PostgreSQL to be ready...
[E2E No-Auth Setup] PostgreSQL is ready
...
[E2E No-Auth Setup] Starting gridapi server in no-auth mode...
[E2E No-Auth Setup] Seeding test data with gridctl...
[E2E No-Auth Setup] ✓ Test data seeded
...
[E2E No-Auth Setup] Starting webapp dev server...
...
[E2E No-Auth Setup] All no-auth services started successfully!

Running 1 test using 1 worker
  ✓  [no-auth] tests/e2e/no-auth-flow.spec.ts:13:1 › No-Auth Flows › should load the graph and display seeded data (XXms)

✓  1 passed (XXs)
[E2E No-Auth Setup] Cleaning up E2E no-auth test services...
```

## 4. How It Works

The `make test-e2e-auth` command triggers the original auth flows script with dedicated `playwright.config.auth.ts` configuration and `make test-e2e` now triggers the following process:

1.  **Builds** `gridapi` and `gridctl` binaries.
2.  **Executes** the `pnpm test:e2e` script with updated `playwright.config.ts` configuration.
3.  **Playwright** starts, invoking the `tests/e2e/setup/start-no-auth-services.sh` script as its `webServer`.
4.  The setup script starts a **PostgreSQL** container, runs **migrations**, and starts the **`gridapi`** server without any authentication environment variables.
5.  The script then uses **`gridctl`** to create a simple graph of test states and dependencies.
6.  Finally, the **webapp** dev server is started.
7.  Once all services are healthy, Playwright runs the tests defined in `tests/e2e/no-auth-flow.spec.ts`.
8.  The test navigates to the webapp, confirms no login is required, and verifies the seeded graph is visible.
9.  After the tests complete, the `trap` in the setup script automatically shuts down all started processes.
