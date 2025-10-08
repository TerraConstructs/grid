# Quickstart: Live Dashboard Integration

**Feature**: 004-wire-up-grid
**Date**: 2025-10-06
**Purpose**: Validate end-to-end integration of dashboard → SDK → API server

---

## Prerequisites

### Environment Setup

1. **PostgreSQL Database Running**:
   ```bash
   make db-up
   # Verify: psql postgres://grid:gridpass@localhost:5432/grid?sslmode=disable
   ```

2. **Grid API Server Running**:
   ```bash
   make build
   ./bin/gridapi db init --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
   ./bin/gridapi db migrate --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
   ./bin/gridapi serve --server-addr localhost:8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
   # Verify: curl http://localhost:8080/healthz
   ```

3. **Test Data Seeded**:
   ```bash
   # Create sample states and dependencies
   ./bin/gridctl state create "prod/network" --server=http://localhost:8080
   ./bin/gridctl state create "prod/cluster" --server=http://localhost:8080 --force # overwrite .grid if exists
   ./bin/gridctl state create "prod/db" --server=http://localhost:8080  --force # overwrite .grid if exists
   ./bin/gridctl deps add --from=prod/network --output=vpc_id --to=prod/cluster --server=http://localhost:8080
   ./bin/gridctl deps add --from=prod/network --output=data_subnets --to=prod/db --server=http://localhost:8080
   ```

4. **TypeScript SDK Built**:
   ```bash
   cd js/sdk
   pnpm install
   pnpm run build
   # Verify: ls lib/esm lib/cjs lib/types
   ```

5. **Webapp Dependencies Installed**:
   ```bash
   cd webapp
   pnpm install
   # Verify @tcons/grid workspace dependency resolves
   ```

---

## Test Scenarios

<!-- Use DevTools MCP -->

### Scenario 1: Dashboard Initial Load

**Goal**: Verify dashboard fetches and displays live states from API server.

**Steps**:
1. Start webapp dev server:
   ```bash
   cd webapp
   VITE_GRID_API_URL=http://localhost:8080 pnpm run dev
   ```

2. Open browser to `http://localhost:5173`

3. Observe loading spinner appears briefly

4. Verify states displayed in graph view:
   - **Expected**: 3 state nodes (prod/network, prod/cluster, prod/db)
   - **Expected**: Nodes positioned in topological layers (network → cluster/db)
   - **Expected**: Node status indicators show correct colors

5. Verify edges displayed:
   - **Expected**: 2 edges connecting states
   - **Expected**: Edge colors match status (clean=green, dirty=orange, etc.)

**Acceptance Criteria**:
- ✅ No mock data displayed (check network tab for real API calls)
- ✅ Loading state transitions to success state
- ✅ Graph renders with correct topology
- ✅ Console shows no errors

---

### Scenario 2: List View Display

**Goal**: Verify list view shows all states and edges from API.

**Steps**:
1. In dashboard, click "List" view button

2. Verify States table:
   - **Expected**: 3 rows (prod/network, prod/cluster, prod/db)
   - **Expected**: Columns show logic_id, GUID prefix, dependencies count, outputs count
   - **Expected**: Status icons match computed_status (clean/stale indicators)

3. Scroll down to Dependency Edges table:
   - **Expected**: 2 rows
   - **Expected**: Columns show from_logic_id, from_output, to_logic_id, status
   - **Expected**: Status badges use correct colors

4. Click on a state row:
   - **Expected**: Detail drawer opens with full state info

**Acceptance Criteria**:
- ✅ All states fetched from ListStates RPC
- ✅ All edges fetched from ListAllEdges RPC
- ✅ Table sorting/interaction works
- ✅ Status colors consistent across views

---

### Scenario 3: Detail View with Dependencies

**Goal**: Verify detail drawer shows comprehensive state information.

**Steps**:
1. In graph or list view, click "prod/cluster" state

2. Verify Overview tab displays:
   - **Expected**: GUID, logic_id, creation/update timestamps
   - **Expected**: Status badge (clean/stale)
   - **Expected**: Backend configuration URLs shown
   - **Expected**: Outputs list (cluster_node_sg, cluster_endpoint, etc.)
   - **Expected**: Sensitive outputs marked with indicator

3. Click "Dependencies" tab:
   - **Expected**: Shows incoming edges from prod/network
   - **Expected**: Edge status colors correct (dirty/clean)
   - **Expected**: from_output → to_input_name mapping shown

4. Click "Dependents" tab:
   - **Expected**: Shows outgoing edges to consumer states (if any)

5. Click on a dependency logic_id link:
   - **Expected**: Navigates to that state's detail view

**Acceptance Criteria**:
- ✅ GetStateInfo RPC called with correct logic_id
- ✅ All fields populated with live data (not mock values)
- ✅ Navigation between states works
- ✅ Sensitive outputs NOT displaying values (only keys + flag)

---

### Scenario 4: Manual Refresh

**Goal**: Verify manual refresh updates dashboard with latest data.

**Steps**:
1. In dashboard, note current state count (3 states)

2. In separate terminal, create new state:
   ```bash
   ./bin/gridctl state create "prod/monitoring" --server=http://localhost:8080
   ```

3. In dashboard header, click refresh button (if implemented) OR reload page

4. Verify new state appears:
   - **Expected**: 4 states now displayed
   - **Expected**: prod/monitoring node visible in graph

5. Verify state count badge updated in header

**Acceptance Criteria**:
- ✅ Refresh triggers new ListStates + ListAllEdges calls
- ✅ New data replaces old data in UI
- ✅ User's current view preserved (selected state, active tab)
- ✅ Loading indicator shown during refresh

---

### Scenario 5: Error Handling

**Goal**: Verify graceful error handling when API unavailable.

**Steps**:
1. Stop Grid API server:
   ```bash
   # Ctrl+C on gridapi serve process
   ```

2. In dashboard, click refresh OR navigate to different state

3. Verify error displayed:
   - **Expected**: Toast/banner notification appears
   - **Expected**: Message: "Service unavailable. Please try again..."
   - **Expected**: Retry button shown

4. Verify partial failure scenario:
   - Restart API server
   - Comment out ListAllEdges handler (simulate missing RPC)
   - Reload dashboard
   - **Expected**: States load, edges fail gracefully
   - **Expected**: Warning indicator shown: "Unable to load edges"
   - **Expected**: Graph view shows states without edges

**Acceptance Criteria**:
- ✅ Error messages are human-readable (not gRPC technical codes)
- ✅ UI doesn't crash on error
- ✅ Retry mechanism works
- ✅ Partial data displayed when possible

---

### Scenario 6: Edge Status Visualization

**Goal**: Verify all edge status types render with correct colors.

**Steps**:
1. Create edges with different statuses:
   ```bash
   # Clean edge
   ./bin/gridctl deps add --from=prod/network --output=vpc_id --to=prod/cluster
   # Mock edge (with mock value)
   ./bin/gridctl deps add --from=prod/network --output=missing_output --to=prod/db --mock='{"value": "placeholder"}'
   ```

2. Refresh dashboard

3. Verify edge colors in graph view:
   - **clean** edges → Green (#10b981)
   - **dirty** edges → Orange/Yellow (#f59e0b)
   - **pending** edges → Blue (#3b82f6)
   - **potentially-stale** edges → Orange/Yellow (#f59e0b)
   - **mock** edges → Purple (#8B5CF6)
   - **missing-output** edges → Red (error color)

4. Hover over edges in graph:
   - **Expected**: Tooltip shows status name + output key

5. In list view, verify status badges use same colors

**Acceptance Criteria**:
- ✅ All 6 status types visually distinct
- ✅ Colors match specification (from clarifications session)
- ✅ Legend/key shown explaining status colors
- ✅ Status consistent across graph and list views

---

### Scenario 7: Empty State Handling

**Goal**: Verify dashboard handles empty/no-data scenarios gracefully.

**Steps**:
1. Clear all states from database:
   ```bash
   make db-reset
   ./bin/gridapi db init --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
   ./bin/gridapi db migrate --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
   ```

2. Reload dashboard

3. Verify empty states:
   - **Expected**: Graph view shows "No states available" message
   - **Expected**: List view shows empty tables with helpful text
   - **Expected**: No JavaScript errors in console

4. Create single state (no dependencies):
   ```bash
   ./bin/gridctl state create --logic-id="dev/standalone"
   ```

5. Refresh dashboard:
   - **Expected**: Single node in graph (isolated)
   - **Expected**: Dependencies/Dependents tabs show "No dependencies"

**Acceptance Criteria**:
- ✅ Empty states don't cause UI errors
- ✅ Helpful messages guide user
- ✅ UI remains functional when data appears

---

## Performance Validation

### Metrics to Measure

1. **Dashboard Initial Load**:
   ```
   Acceptable: < 2 seconds (3 states, 8 edges)
   Good:       < 1 second
   Measure:    Chrome DevTools Network tab → DOMContentLoaded
   ```

2. **API Response Times**:
   ```
   ListStates:     < 100ms (50 states)
   GetStateInfo:   < 50ms
   ListAllEdges:   < 200ms (200 edges)
   Measure:        gridapi logs or curl -w "@time-template.txt"
   ```

3. **Graph Rendering**:
   ```
   Acceptable: < 500ms (50 states, 100 edges)
   Measure:    Chrome DevTools Performance tab
   ```

4. **Bundle Size** (tree-shaking verification):
   ```bash
   cd webapp
   pnpm run build
   ls -lh dist/assets/*.js
   # Acceptable: < 500KB for main bundle (gzipped)
   # Verify only ListStates, GetStateInfo, ListAllEdges methods included
   ```

**Performance Acceptance Criteria**:
- ✅ Initial load < 2s
- ✅ API calls < 200ms p95
- ✅ Graph render < 500ms
- ✅ Bundle size < 500KB (gzipped)

---

## Rollback Plan

If issues found during validation:

1. **Revert to Mock Data** (temporary):
   ```typescript
   // webapp/src/main.tsx
   import { mockApi } from './services/mockApi';
   // Use mockApi instead of GridApiAdapter until fixed
   ```

2. **Isolate SDK Issues**:
   ```bash
   cd js/sdk
   pnpm test  # Run SDK unit tests
   ```

3. **Isolate API Issues**:
   ```bash
   # Test ListAllEdges RPC directly
   curl -X POST http://localhost:8080/state.v1.StateService/ListAllEdges \
     -H "Content-Type: application/json" \
     -d '{}'
   ```

4. **Check Backend Logs**:
   ```bash
   # gridapi serve output shows request/response logs
   # Look for errors, slow queries, invalid responses
   ```

---

## Success Criteria Summary

**This quickstart is considered successful when**:

✅ All 7 test scenarios pass without errors
✅ Dashboard displays live data (verified via network tab showing Connect RPC calls)
✅ Performance metrics within acceptable ranges
✅ Error handling works gracefully (Scenario 5)
✅ Visual consistency (colors, layouts) matches specification
✅ No console errors or warnings
✅ Build artifacts generated correctly (SDK lib/, webapp dist/)

**Integration is complete when**:
- mockApi.ts file can be deleted
- All components import types from `@tcons/grid`
- API server handles ListAllEdges RPC
- Repository test coverage ≥80%

---

## Next Steps

After quickstart validation:
1. Run full test suite: `make test-all`
2. Update CLAUDE.md with new SDK patterns
3. Document any deviations or issues found
4. Create PR with passing CI checks

---

**Status**: Ready for Execution
**Estimated Time**: 30-45 minutes
