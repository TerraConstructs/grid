# E2E Test Expansion: Read-Only Webapp Validation (No-Auth Mode)

**Status:** Proposed
**Created:** 2025-11-25
**Related:** `specs/007-webapp-auth/`, `tests/e2e/default-flow.spec.ts`

---

## Overview

Expand E2E test coverage for the read-only Grid webapp by validating visualization and navigation features. Tests focus on **verifying UI representation** of infrastructure state managed via CLI/API, not ClickOps workflows.

### Core Principle
- **Setup via CLI/API/SDK** (gridctl, Terraform HTTP backend, Go SDK, curl)
- **Verify via Webapp** (visual validation, navigation, filtering)
- **Cleanup per test** (isolated database state, no cross-test pollution)

---

## Test Architecture

### Test Execution Pattern

Each E2E test follows this pattern:

```typescript
test('scenario name', async ({ page }) => {
  // SETUP PHASE (before navigation)
  // - Create states via gridctl/SDK
  // - Upload tfstate via HTTP backend
  // - Add dependencies via gridctl

  // VERIFICATION PHASE (webapp interaction)
  await page.goto('/');
  // - Navigate UI
  // - Verify visual elements
  // - Check data-testid selectors

  // CLEANUP PHASE (test.afterEach)
  // - Delete states via SDK
  // - Truncate database tables (if needed)
});
```

### Setup Methods (3 Approaches)

#### **Option 1: Direct SDK Calls in Test (Recommended)**

Note: required changes to js/sdk -> [js-sdk-fixes.md](specs/009-e2e-noauth/js-sdk-fixes.md)

```typescript
import { sdk } from '@tcons/grid'; // Generated TypeScript SDK

test('edge status visualization', async ({ page }) => {
  const client = sdk.createClient({ baseUrl: 'http://localhost:8080' });

  // Setup: Create states
  const producer = await client.state.create({ logicId: 'test-producer' });
  const consumer = await client.state.create({ logicId: 'test-consumer' });

  // Setup: Add dependency
  await client.dependency.add({
    from: { logicId: 'test-producer' },
    fromOutput: 'vpc_id',
    to: { logicId: 'test-consumer' },
  });

  // Verify: Navigate to webapp
  await page.goto('/');
  const edge = page.locator('[data-testid^="graph-edge-"]').first();
  await expect(edge).toHaveAttribute('stroke', /#3b82f6/); // Blue = pending

  // Cleanup happens in afterEach hook
});

test.afterEach(async () => {
  const client = sdk.createClient({ baseUrl: 'http://localhost:8080' });
  // Delete all states created in this test
  await client.state.deleteByPattern({ logicIdPrefix: 'test-' });
});
```

**Pros:**
- TypeScript type safety
- Direct integration with test code
- Easy to debug
- Access to response data (GUIDs, IDs)

**Cons:**
- Requires TypeScript SDK generation
- More verbose than shell scripts

---

#### **Option 2: Bash Helper Scripts**
```typescript
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

test('edge status visualization', async ({ page }) => {
  // Setup: Run helper script
  const { stdout } = await execAsync('./tests/e2e/helpers/seed-edge-status-test.sh');
  const { producerGuid, consumerGuid } = JSON.parse(stdout);

  // Verify: Navigate to webapp
  await page.goto('/');
  // ... assertions

  // Cleanup: Run cleanup script
  await execAsync(`./tests/e2e/helpers/cleanup-test.sh ${producerGuid} ${consumerGuid}`);
});
```

**Helper script example:**
```bash
#!/usr/bin/env bash
# tests/e2e/helpers/seed-edge-status-test.sh

export GRID_SERVER_URL="http://localhost:8080"

# Create states
PRODUCER_GUID=$(gridctl state create "test-producer-$(date +%s)" --output json | jq -r '.guid')
CONSUMER_GUID=$(gridctl state create "test-consumer-$(date +%s)" --output json | jq -r '.guid')

# Add dependency
gridctl deps add --from "test-producer-*" --output vpc_id --to "test-consumer-*"

# Return GUIDs as JSON
echo "{\"producerGuid\": \"$PRODUCER_GUID\", \"consumerGuid\": \"$CONSUMER_GUID\"}"
```

**Pros:**
- Reusable across tests
- Familiar bash/gridctl patterns
- Easy to prototype

**Cons:**
- Harder to debug failures
- JSON parsing overhead
- Less type safety

---

#### **Option 3: Playwright Fixtures (Test-Scoped Setup)**
```typescript
// tests/e2e/fixtures.ts
import { test as base } from '@playwright/test';
import { sdk } from '@tcons/grid';

type GridFixtures = {
  gridClient: ReturnType<typeof sdk.createClient>;
  cleanupStates: (logicIdPattern: string) => Promise<void>;
};

export const test = base.extend<GridFixtures>({
  gridClient: async ({}, use) => {
    const client = sdk.createClient({ baseUrl: 'http://localhost:8080' });
    await use(client);
  },

  cleanupStates: async ({ gridClient }, use) => {
    const createdStates: string[] = [];

    const cleanup = async (pattern: string) => {
      createdStates.push(pattern);
    };

    await use(cleanup);

    // Cleanup after test completes
    for (const pattern of createdStates) {
      await gridClient.state.deleteByPattern({ logicIdPrefix: pattern });
    }
  },
});

// Usage in tests:
test('edge status visualization', async ({ page, gridClient, cleanupStates }) => {
  // Setup
  const producer = await gridClient.state.create({ logicId: 'test-producer-edge' });
  const consumer = await gridClient.state.create({ logicId: 'test-consumer-edge' });
  await cleanupStates('test-producer-edge');
  await cleanupStates('test-consumer-edge');

  // ... test assertions

  // Automatic cleanup via fixture
});
```

**Pros:**
- Automatic cleanup (even on test failure)
- Reusable client setup
- Clean test code
- Playwright-native pattern

**Cons:**
- More upfront boilerplate
- Fixtures shared across tests (careful with state)

---

### **Recommended Approach: Hybrid**

Use **Option 1 (SDK) + Option 3 (Fixtures)** for production tests:

```typescript
// tests/e2e/fixtures/grid-client.fixture.ts
import { test as base, expect } from '@playwright/test';
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { StateService } from '@tcons/grid/gen/state/v1/state_connect';

export const test = base.extend({
  gridClient: async ({}, use) => {
    const transport = createConnectTransport({
      baseUrl: 'http://localhost:8080',
    });
    const client = createPromiseClient(StateService, transport);
    await use(client);
  },
});

export { expect };
```

```typescript
// tests/e2e/edge-status.spec.ts
import { test, expect } from './fixtures/grid-client.fixture';

test.describe('Edge Status Visualization', () => {
  let producerGuid: string;
  let consumerGuid: string;

  test.beforeEach(async ({ gridClient }) => {
    // Setup: Create states
    const producer = await gridClient.createState({
      guid: '', // Server generates
      logicId: `test-producer-${Date.now()}`,
    });
    producerGuid = producer.guid;

    const consumer = await gridClient.createState({
      guid: '',
      logicId: `test-consumer-${Date.now()}`,
    });
    consumerGuid = consumer.guid;

    // Setup: Add dependency
    await gridClient.addDependency({
      from: { logicId: producer.logicId },
      fromOutput: 'vpc_id',
      to: { logicId: consumer.logicId },
    });
  });

  test.afterEach(async ({ gridClient }) => {
    // Cleanup: Delete states
    if (producerGuid) {
      await gridClient.deleteState({ guid: producerGuid }).catch(() => {});
    }
    if (consumerGuid) {
      await gridClient.deleteState({ guid: consumerGuid }).catch(() => {});
    }
  });

  test('should show pending edge as blue', async ({ page }) => {
    await page.goto('/');

    const edge = page.locator('[data-testid^="graph-edge-"]:not([data-testid*="-hover-"])').first();

    // Pending edge should be blue (#3b82f6)
    await expect(edge).toHaveAttribute('stroke', /#3b82f6/);
  });

  test('should transition to dirty (amber) after producer upload', async ({ page, gridClient }) => {
    // Upload tfstate to producer
    const tfstate = {
      version: 4,
      terraform_version: '1.6.0',
      serial: 1,
      outputs: {
        vpc_id: { value: 'vpc-12345', type: 'string' },
      },
    };

    // Use fetch since it's browser-compatible
    await fetch(`http://localhost:8080/tfstate/${producerGuid}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(tfstate),
    });

    // Wait for edge update job
    await page.waitForTimeout(1000);

    await page.goto('/');

    const edge = page.locator('[data-testid^="graph-edge-"]:not([data-testid*="-hover-"])').first();

    // Dirty edge should be amber (#f59e0b)
    await expect(edge).toHaveAttribute('stroke', /#f59e0b/);
  });
});
```

---

## Test Scenarios (Priority Order)

### **Priority 1: Edge Status Lifecycle** ⭐⭐⭐

**Integration Test Equivalent:** `TestEdgeStatusTracking`, `TestEdgeUpdateAndStatusComputation`

**Test File:** `tests/e2e/edge-status.spec.ts`

**Scenario:**
1. Create producer + consumer states (SDK)
2. Add dependency (SDK) → edge status: **pending** (blue)
3. Upload producer tfstate (HTTP POST) → edge status: **dirty** (amber)
4. Upload consumer tfstate (HTTP POST) → edge status: **clean** (green)
5. Update producer tfstate (HTTP POST) → edge status: **stale** (red)

**Assertions:**
- Edge `stroke` attribute matches status color
- Edge tooltip shows correct status text
- List view shows matching status badge

**Setup/Cleanup:**
- `beforeEach`: Create states via SDK
- `afterEach`: Delete states via SDK (even on failure)

---

### **Priority 2: Label Filtering** ⭐⭐⭐

**Integration Test Equivalent:** `TestLabelFiltering`

**Test File:** `tests/e2e/label-filtering.spec.ts`

**Scenario:**
1. Create 3 states with labels (SDK):
   - `state-prod-platform`: `env=prod, team=platform`
   - `state-dev-platform`: `env=dev, team=platform`
   - `state-prod-data`: `env=prod, team=data`
2. Navigate to webapp
3. Apply filter: `env=prod`
4. Verify only 2 states visible
5. Add filter: `team=platform`
6. Verify only 1 state visible (intersection)

**Assertions:**
- `graph-node-*` count matches filter
- `list-state-row-*` count matches filter
- Filter input shows active filters

**Setup/Cleanup:**
- `beforeEach`: Create 3 labeled states
- `afterEach`: Delete all test states

---

### **Priority 3: Graph ↔ List View Consistency** ⭐⭐

**Integration Test Equivalent:** `TestDependencyListingAndStatus`

**Test File:** `tests/e2e/view-consistency.spec.ts`

**Scenario:**
1. Create 5 states with 4 dependencies (SDK)
2. Navigate to graph view
3. Verify 5 nodes, 4 edges visible
4. Switch to list view
5. Verify 5 state rows, 4 edge rows
6. Switch back to graph view
7. Verify data persists

**Assertions:**
- Node/edge counts match in both views
- Same states visible in both views
- View toggle preserves data

**Setup/Cleanup:**
- `beforeEach`: Create 5-state topology
- `afterEach`: Delete all states

---

### **Priority 4: Detail View Navigation** ⭐⭐

**Integration Test Equivalent:** `TestTopologicalOrdering`

**Test File:** `tests/e2e/detail-view.spec.ts`

**Scenario:**
1. Create 4-layer chain: `foundation → network → compute → app` (SDK)
2. Click `app` node in graph
3. Detail view opens showing dependencies
4. Click `compute` dependency link
5. Detail view switches to `compute`
6. Verify dependencies tab shows `network`

**Assertions:**
- `detail-modal-{logicId}` visible for clicked state
- `detail-dependency-*` links navigate correctly
- Close button returns to graph

**Setup/Cleanup:**
- `beforeEach`: Create 4-state chain
- `afterEach`: Delete all states

---

### **Priority 5: Terraform Outputs Display** ⭐

**Integration Test Equivalent:** `TestQuickstartTerraform`, `TestSchemaWithDependencies`

**Test File:** `tests/e2e/terraform-outputs.spec.ts`

**Scenario:**
1. Create state `landing-zone` (SDK)
2. Upload tfstate with outputs (HTTP POST):
   ```json
   {
     "outputs": {
       "vpc_id": { "value": "vpc-12345", "type": "string" },
       "subnet_ids": { "value": ["subnet-a", "subnet-b"], "type": "list(string)" }
     }
   }
   ```
3. Click `landing-zone` node
4. Navigate to "Outputs" tab in detail view
5. Verify outputs displayed

**Assertions:**
- Output keys visible (`vpc_id`, `subnet_ids`)
- Output values visible (`vpc-12345`, `["subnet-a", "subnet-b"]`)
- Output types visible (`string`, `list(string)`)

**Setup/Cleanup:**
- `beforeEach`: Create state + upload tfstate
- `afterEach`: Delete state

---

### **Priority 6: Manual Refresh** ⭐

**Integration Test Equivalent:** `TestRestartPersistence`

**Test File:** `tests/e2e/refresh.spec.ts`

**Scenario:**
1. Create 2 states (SDK)
2. Navigate to webapp
3. Verify 2 nodes visible
4. Create 3rd state in background (SDK, simulates another user)
5. Click refresh button
6. Verify 3 nodes visible

**Assertions:**
- Initial: 2 nodes
- After refresh: 3 nodes
- New node has correct `data-testid`

**Setup/Cleanup:**
- `beforeEach`: Create 2 states
- `afterEach`: Delete all states (including 3rd)

---

### **Priority 7: Large Graph Rendering** ⭐

**Integration Test Equivalent:** `TestGetDependencyGraph`

**Test File:** `tests/e2e/large-graph.spec.ts`

**Scenario:**
1. Create 20 states with diamond/chain patterns (SDK)
2. Navigate to webapp
3. Verify all 20 nodes render
4. Test zoom in/out controls
5. Test fit view control
6. Click node to open detail view

**Assertions:**
- 20 nodes visible
- Zoom controls functional
- Fit view centers graph
- Node selection works

**Setup/Cleanup:**
- `beforeEach`: Create 20-state topology (slow, maybe use seeded data)
- `afterEach`: Delete all states

---

### **Priority 8: Error Handling** ⭐

**Integration Test Equivalent:** `TestNonExistentStateAccess`

**Test File:** `tests/e2e/error-handling.spec.ts`

**Scenario:**
1. Create state `temp-state` (SDK)
2. Navigate to webapp
3. Click `temp-state` node to open detail view
4. Delete state in background (SDK)
5. Refresh detail view or click again
6. Verify error notification appears

**Assertions:**
- `notification-toast-*` visible
- Error message contains "not found" or similar
- UI doesn't crash

**Setup/Cleanup:**
- `beforeEach`: Create state
- `afterEach`: Delete state if still exists

---

## File Structure

```
tests/e2e/
├── fixtures/
│   ├── grid-client.fixture.ts       # SDK client fixture
│   └── cleanup.fixture.ts           # Auto-cleanup fixture
├── helpers/
│   ├── auth.helpers.ts              # Existing auth helpers
│   ├── state.helpers.ts             # Existing state helpers
│   └── tfstate.helpers.ts           # NEW: Helper to upload tfstate
├── default-flow.spec.ts             # Existing smoke test
├── edge-status.spec.ts              # Priority 1
├── label-filtering.spec.ts          # Priority 2
├── view-consistency.spec.ts         # Priority 3
├── detail-view.spec.ts              # Priority 4
├── terraform-outputs.spec.ts        # Priority 5
├── refresh.spec.ts                  # Priority 6
├── large-graph.spec.ts              # Priority 7
└── error-handling.spec.ts           # Priority 8
```

---

## Cleanup Strategy

### **Per-Test Cleanup (Isolated State)**

Each test must clean up its own states to prevent cross-test pollution:

```typescript
test.afterEach(async ({ gridClient }) => {
  // Delete states created in this test
  const statesToDelete = ['test-producer-edge', 'test-consumer-edge'];

  for (const logicId of statesToDelete) {
    try {
      const state = await gridClient.getState({ logicId });
      await gridClient.deleteState({ guid: state.guid });
    } catch (err) {
      // State might not exist (test failed early), ignore
      console.log(`Cleanup: ${logicId} not found, skipping`);
    }
  }
});
```

### **Global Cleanup (Fallback)**

If tests fail and leave orphaned data, add a global cleanup:

```typescript
// tests/e2e/global-teardown.ts
import { createPromiseClient } from '@connectrpc/connect';
import { StateService } from '@tcons/grid/gen/state/v1/state_connect';

export default async function globalTeardown() {
  const client = createPromiseClient(StateService, /* ... */);

  // Delete all states with 'test-' prefix
  const states = await client.listStates({ filter: '' });
  for (const state of states.states) {
    if (state.logicId.startsWith('test-')) {
      await client.deleteState({ guid: state.guid });
    }
  }
}
```

**Configure in `playwright.config.ts`:**
```typescript
export default defineConfig({
  globalTeardown: './tests/e2e/global-teardown.ts',
  // ...
});
```

---

## Database Isolation

### **Shared Database Challenges**

Since all tests share the same PostgreSQL database:

1. **Unique naming:** Use timestamps or UUIDs in logic_ids
2. **Cleanup on failure:** Use `try/finally` or Playwright fixtures
3. **No parallel execution:** Run no-auth tests serially (`workers: 1`)

### **Naming Convention**

```typescript
// ✅ Good: Unique per test run
const logicId = `test-producer-${Date.now()}-${Math.random().toString(36).slice(2)}`;

// ❌ Bad: Collision risk
const logicId = 'test-producer';
```

### **Playwright Config (No Parallel)**

```typescript
// playwright.config.ts
export default defineConfig({
  workers: 1, // Serial execution to avoid DB conflicts
  // ...
});
```

---

## HTTP Backend Helpers

### **Upload Tfstate Helper**

```typescript
// tests/e2e/helpers/tfstate.helpers.ts
export async function uploadTfstate(guid: string, outputs: Record<string, any>) {
  const tfstate = {
    version: 4,
    terraform_version: '1.6.0',
    serial: 1,
    outputs: Object.fromEntries(
      Object.entries(outputs).map(([key, value]) => [
        key,
        { value, type: typeof value === 'string' ? 'string' : 'any' },
      ])
    ),
  };

  const response = await fetch(`http://localhost:8080/tfstate/${guid}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(tfstate),
  });

  if (!response.ok) {
    throw new Error(`Failed to upload tfstate: ${response.statusText}`);
  }

  // Wait for edge update job to process
  await new Promise(resolve => setTimeout(resolve, 500));
}
```

**Usage:**
```typescript
test('edge becomes dirty after tfstate upload', async ({ page, gridClient }) => {
  const producer = await gridClient.createState({ logicId: 'test-producer' });

  // Upload tfstate with outputs
  await uploadTfstate(producer.guid, { vpc_id: 'vpc-12345' });

  // Verify edge color changed
  await page.goto('/');
  const edge = page.locator('[data-testid^="graph-edge-"]').first();
  await expect(edge).toHaveAttribute('stroke', /#f59e0b/); // Amber = dirty
});
```

---

## Implementation Phases

### **Phase 1: Foundation (Week 1)**
- ✅ Create SDK fixture (`grid-client.fixture.ts`)
- ✅ Create tfstate helper (`tfstate.helpers.ts`)
- ✅ Implement Priority 1: Edge Status Lifecycle (1 test file)
- ✅ Verify cleanup strategy works

### **Phase 2: Core Features (Week 2)**
- ✅ Priority 2: Label Filtering
- ✅ Priority 3: View Consistency
- ✅ Priority 4: Detail View Navigation

### **Phase 3: Advanced Features (Week 3)**
- ✅ Priority 5: Terraform Outputs
- ✅ Priority 6: Manual Refresh
- ✅ Priority 7: Large Graph Rendering

### **Phase 4: Edge Cases (Week 4)**
- ✅ Priority 8: Error Handling
- ✅ Global teardown script
- ✅ CI/CD integration

---

## Success Metrics

- **Coverage:** 8 new test files covering main webapp features
- **Reliability:** Tests pass consistently (no flakiness from shared DB)
- **Cleanup:** Zero orphaned states after test runs
- **Performance:** Test suite completes in <5 minutes
- **Maintainability:** Clear separation between setup (SDK) and verification (UI)

---

## Open Questions

1. **SDK Generation:** Do we have TypeScript SDK generated from protobuf? (Check `js/sdk/gen/`)
2. **Delete API:** Does `StateService` have a `DeleteState` RPC method for cleanup?
3. **Parallel Execution:** Can we use PostgreSQL transactions/schemas for test isolation?
4. **Test Data:** Should we seed a "large graph" dataset once, or generate per test?
5. **Edge Update Job:** Is 500ms wait sufficient for edge status updates, or should we poll?

---

## References

- Integration Tests: `tests/integration/dependency_test.go`, `context_aware_test.go`
- Current E2E Test: `tests/e2e/default-flow.spec.ts`
- Webapp Components: `webapp/src/components/GraphView.tsx`, `ListView.tsx`
- Data-testid PR: `fix(e2e): adopt data-testid for reliable test selectors` (commit 4425443)
