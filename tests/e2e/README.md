# Grid E2E Tests

End-to-end tests for the Grid webapp using Playwright with TypeScript.

## Overview

These tests validate the complete authentication and authorization flow through the webapp UI:
- OAuth2/OIDC with Keycloak
- Session management and persistence
- Group-based RBAC (Role-Based Access Control)
- Just-In-Time (JIT) user provisioning
- State creation with label-scoped permissions

## Prerequisites

- Node.js 20+
- pnpm 10+
- Docker (for running services)
- jq (for extracting secrets from realm export)

## Setup

### 1. Install Dependencies

```bash
# Install npm/pnpm dependencies (includes @playwright/test)
pnpm install

# Install Playwright browsers
npx playwright install chromium
```

### 2. Environment Configuration

The E2E tests use environment variables defined in `.env.e2e.example`. The setup scripts handle most configuration automatically, but you can customize if needed:

```bash
cp .env.e2e.example .env.e2e
# Edit .env.e2e if you need custom values
```

## Running Tests

### Local Development

```bash
# Run all E2E tests (headless)
make test-e2e

# Run tests in UI mode (interactive)
make test-e2e-ui

# Run tests in headed mode (see browser)
make test-e2e-headed

# Run tests in debug mode
make test-e2e-debug

# View test report after run
make test-e2e-report
```

### Using pnpm Directly

```bash
pnpm test:e2e           # Run all tests
pnpm test:e2e:ui        # Interactive UI mode
pnpm test:e2e:headed    # Headed mode
pnpm test:e2e:debug     # Debug mode
pnpm test:e2e:report    # View report
```

## Test Architecture

### Directory Structure

```
tests/e2e/
├── setup/
│   ├── start-services.sh      # Service orchestration (docker, gridapi, webapp)
│   ├── seed-test-data.sh      # Create test users in Keycloak
│   └── global-setup.ts        # Playwright global setup
├── helpers/
│   ├── auth.helpers.ts        # Login, logout, session management
│   ├── state.helpers.ts       # State creation and verification
│   ├── keycloak.helpers.ts    # Group management, cache refresh
│   └── fixtures.ts            # Playwright test fixtures
├── auth-flow.spec.ts          # Main authentication test scenarios
└── README.md                  # This file
```

### Service Orchestration

The `start-services.sh` script automatically:
1. Starts Docker services (PostgreSQL, Keycloak)
2. Waits for services to be healthy (with retries for Keycloak)
3. Seeds test users in Keycloak
4. Builds gridapi if needed
5. Runs database migrations
6. Bootstraps group-to-role mappings
7. Starts gridapi server (Mode 1 - External IdP)
8. Starts webapp dev server (Vite)

All services are torn down when tests complete.

### Test Users

Tests use pre-configured users from Keycloak:

| Email                    | Password | Groups              | Permissions           |
|--------------------------|----------|---------------------|-----------------------|
| alice@example.com        | test123  | product-engineers   | env=dev states only   |
| platform@example.com     | test123  | platform-engineers  | Full access (wildcard)|
| newuser@example.com      | test123  | (none)              | JIT provisioning test |

## Test Scenarios

### 1. Successful Login and Session Persistence
- Tests OAuth2/OIDC flow with Keycloak
- Validates session cookie persistence across reloads
- Verifies logout functionality

### 2. Group-Based Permission - Allowed Action
- alice@example.com (product-engineer) can create states with `env=dev`
- Verifies success notification and state appears in list

### 3. Group-Based Permission - Forbidden Action
- alice@example.com (product-engineer) CANNOT create states with `env=prod`
- Verifies permission denied error

### 4. First-Time Login JIT Provisioning
- newuser@example.com exists in Keycloak but not in Grid DB
- Login triggers automatic user creation
- Verifies successful authentication

### 5. Platform Engineer Full Access
- platform@example.com has wildcard permissions
- Can create states with any labels (including env=prod)

## Writing New Tests

### Using Test Fixtures

```typescript
import { test, expect } from './helpers/fixtures';

test('example test', async ({ authenticatedPage }) => {
  // authenticatedPage is already logged in as alice@example.com
  await authenticatedPage.goto('/');
  // ... your test code
});
```

### Available Fixtures

- `authenticatedPage` - Logged in as alice@example.com (product-engineer)
- `platformEngineerPage` - Logged in as platform@example.com (full access)
- `newUserPage` - Logged in as newuser@example.com (no groups)

### Using Helpers

```typescript
import { loginViaKeycloak, logout } from './helpers/auth.helpers';
import { createState, verifyStateCreated } from './helpers/state.helpers';
import { refreshIAMCache } from './helpers/keycloak.helpers';

test('custom test', async ({ page }) => {
  // Login manually
  await loginViaKeycloak(page, 'alice@example.com', 'test123');

  // Create a state
  await createState(page, {
    logicId: 'my-state',
    labels: { env: 'dev' },
  });

  // Verify creation
  await verifyStateCreated(page, 'my-state');

  // Logout
  await logout(page);
});
```

## Debugging

### View Browser While Testing

```bash
make test-e2e-headed
```

### Pause and Inspect

```bash
make test-e2e-debug
```

Or add `await page.pause()` in your test code.

### Check Service Logs

Service logs are written to `/tmp/`:
- `/tmp/grid-e2e-gridapi.log` - gridapi server logs
- `/tmp/grid-e2e-webapp.log` - webapp dev server logs

### Common Issues

#### Keycloak Takes Long to Start

Keycloak may take 30-60 seconds to fully start and import the realm.
The script waits up to 120 seconds and shows progress every 10 attempts (every 20 seconds).

**Check Keycloak status:**
```bash
curl http://localhost:8443/health/ready
# Should eventually return: {"status":"UP","checks":[]}
```

**Check Keycloak logs:**
```bash
docker compose logs keycloak | tail -50
```

#### Services Not Starting

Check Docker is running:
```bash
docker compose ps
```

#### Permission Tests Failing

Ensure group-to-role mappings are correct:
```bash
./bin/gridapi iam bootstrap --group product-engineers --role product-engineer --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

Refresh IAM cache if needed:
```bash
kill -HUP $(cat /tmp/grid-e2e-gridapi.pid)
```

## CI/CD Integration

Tests are designed to run in GitHub Actions. See `.github/workflows/` for CI configuration (to be added).

## Related Documentation

- [TESTING.md](../../specs/007-webapp-auth/TESTING.md) - Test scenarios and requirements
- [Playwright Docs](https://playwright.dev/docs/intro) - Official Playwright documentation
- [CLAUDE.md](../../CLAUDE.md) - Project development guidelines
