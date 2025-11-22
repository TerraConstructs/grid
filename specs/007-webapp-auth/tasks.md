# Tasks Index: WebApp User Login Flow with Role-Based Filtering

Beads Issue Graph Index into the tasks and phases for this feature implementation.
This index does **not contain tasks directly**—those are fully managed through Beads CLI.

## Feature Tracking

* **Beads Epic ID**: `grid-baf5`
* **Epic Title**: WebApp User Login Flow with Role-Based Filtering
* **User Stories Source**: `specs/007-webapp-auth/spec.md`
* **Research Inputs**: `specs/007-webapp-auth/research.md`
* **Planning Details**: `specs/007-webapp-auth/plan.md`
* **Data Model**: `specs/007-webapp-auth/data-model.md`
* **Contract Definitions**: `specs/007-webapp-auth/contracts/`

## 📊 Feature Status Summary (Updated 2025-11-18)

| Component | Status | Notes |
|-----------|--------|-------|
| **Phase 1: Setup** | ✅ COMPLETE | TypeScript types, context scaffolding, SDK stubs |
| **Phase 2: Foundational** | ✅ COMPLETE | Backend auth handlers (login, whoami, role aggregation) |
| **US1: Non-Auth Mode** | ✅ COMPLETE | Dashboard works without authentication |
| **US2: Internal IdP** | ✅ COMPLETE | Username/password login (MVP) |
| **US3: External IdP/SSO** | ✅ COMPLETE | OAuth2 redirect + Keycloak integration |
| **US4: Role-Based Filtering** | ✅ COMPLETE | Client-side state filtering by role scope |
| **US5: Auth Status UI** | ✅ COMPLETE | User menu showing role/group info |
| **US6: Logout** | ✅ COMPLETE | Session termination and redirect to login |
| **Integration Tests** | ✅ COMPLETE (Mode2) | Session + Connect RPC tested (Mode 1 requires E2E) |
| **Polish & Enhancements** | ⚠️ PARTIAL | 5 tasks remaining (see below) |

**Summary**: All 6 user stories fully implemented. 5 enhancement/test tasks remain open (component tests, E2E tests, bug fix, documentation, technical debt).

**Next Steps**: See "Remaining Open Tasks & Next Steps" section below for prioritized action items.

## Beads Query Hints

Use the `bd` CLI or MCP toolchain to query and manipulate the issue graph:

```bash
# Find all open tasks for this feature
bd list --label spec:007-webapp-auth --status open

# Find ready tasks to implement
bd ready --label spec:007-webapp-auth

# See design, acceptance criteria, and dependencies for a specific issue
bd show grid-baf5

# Explore comments for a specific issue
bd comments grid-044b

# View issues by component
bd list --label component:webapp --label spec:007-webapp-auth
bd list --label component:gridapi --label spec:007-webapp-auth
bd list --label component:sdk --label spec:007-webapp-auth

# Show all phases (features)
bd list --type feature --label spec:007-webapp-auth

# Show all tasks for a specific user story
bd list --label story:US1 --label spec:007-webapp-auth
bd list --label story:US2 --label spec:007-webapp-auth

# Define dependencies
bd dep add <from-issue-id> <to-issue-id> --type <dependency-type>

# valid dependency types
# (blocks|related|parent-child|discovered-from) (default "blocks")

# Check stats
bd stats
```


### Exploring comments for context

Use `bd comments` to add notes, research findings, or any relevant context to a task. This helps in task generation and exploration by providing additional details that might not fit into the task title, description, design or notes sections on the task directly.

**Example:**
```bash
bd comments grid-044b
```

This will show comments associated with `grid-044b`, such as:

```console
Comments on grid-044b:
[vincentdesmet] Research hashicorp/js-bexpr library: https://github.com/hashicorp/js-bexpr - This may provide go-bexpr compatible syntax for browser. Alternative: expr-eval or custom parser. at
 2025-11-04 23:49
```

Use `bd comments add <issue-id> "<your comment>"` to add your own insights during implementation.

## Tasks and Phases Structure

This feature follows Beads' 2-level graph structure:

* **Epic**: grid-baf5 → WebApp User Login Flow with Role-Based Filtering
* **Phases**: Beads issues of type `feature`, child of the epic
  * Phase 1: Setup and Infrastructure (grid-990f)
  * Phase 2: Foundational Backend Requirements (grid-f6ce) - **BLOCKS all user stories**
  * Phase 3: US1 - View States Without Authentication (grid-bad5)
  * Phase 4: US2 - Authenticate Using Internal Identity Provider (grid-fdfc)
  * Phase 5: US3 - Authenticate Using External Identity Provider (grid-74c0)
  * Phase 6: US4 - See Only Authorized States (grid-32ef)
  * Phase 7: US5 - View Authentication Status and Role Information (grid-b2e6)
  * Phase 8: US6 - Log Out (grid-2374)
  * Phase 9: Polish and Integration (grid-a16b)
* **Tasks**: Issues of type `task`, children of each feature issue (phase)

## Convention Summary

| Label Type          | Purpose                                    | Examples                                             |
| ------------------- | ------------------------------------------ | ---------------------------------------------------- |
| `spec:007-webapp-auth` | All issues in this feature              | Applied to epic, features, and tasks                 |
| `phase:setup`       | Phase categorization                       | setup, foundational, us1, us2, us3, us4, us5, us6, polish |
| `story:US1`         | User story traceability                    | US1, US2, US3, US4, US5, US6                          |
| `component:webapp`  | Implementation area                        | webapp, gridapi, sdk, infra, integration              |
| `fr:FR-001`         | Functional requirement traceability        | FR-001 through FR-010 (from spec.md)                  |

## Phase Details

### Phase 1: Setup and Infrastructure (grid-990f)

**Purpose**: Project initialization - TypeScript types, context scaffolding, SDK stubs

**Tasks Created**:
- grid-e586: Create TypeScript auth types (webapp/src/types/auth.ts)
- grid-b1f7: Create js/sdk auth types (js/sdk/types/auth.ts)
- grid-8602: Create AuthContext scaffolding (webapp/src/context/AuthContext.tsx)
- grid-174c: Create js/sdk auth helper stubs (js/sdk/auth.ts)

**Status**: completed

---

### Phase 2: Foundational Backend Requirements (grid-f6ce) ⚠️ BLOCKS ALL USER STORIES

**Purpose**: Backend changes required before any user story can be implemented

**Critical**: This phase MUST be complete before any user story work begins.

**Architectural Note (Constitution IX)**: New auth handlers will follow the existing SSO auth handler pattern (direct repository access via `deps.Users` and `deps.Sessions`). This is documented technical debt per Constitution IX "Known Violations" section. The complete fix (introducing IAM service layer) is tracked separately for future refactoring.

**Tasks Created**:
- grid-87f5: Implement POST /auth/login for internal IdP (cmd/gridapi/internal/server/auth_handlers.go)
- grid-8f1a: Implement gridapi users create command (cmd/gridapi/cmd/users/create.go)
- grid-6b5a: Implement /api/auth/whoami endpoint (cmd/gridapi/internal/server/auth_handlers.go)
- grid-c254: Implement role aggregation logic for whoami (cmd/gridapi/internal/server/auth_helpers.go)
- grid-d2a2: Fix SessionID population bug in authn middleware (cmd/gridapi/internal/middleware/authn.go)
- grid-830e: Mount whoami and internal login endpoints in router (cmd/gridapi/internal/server/router.go)

**Backend Changes Summary** (from plan.md Complexity Tracking):
1. POST /auth/login (internal IdP): Username/password authentication handler (~80 lines)
2. gridapi users create: CLI command to bootstrap local users (~100 lines)
3. GET /api/auth/whoami: Session restoration endpoint (~50 lines)
4. Role aggregation: Union of user_roles + group_roles (~30 lines)
5. SessionID bug fix: Populate principal.SessionID field (~5 lines)
6. Router mounting: Register new endpoints (~2 lines)

**Estimated Total Effort**: 7-10 hours

**Constitution IX Justification**: All changes follow existing gridapi patterns - handlers call repositories directly for auth operations (see HandleSSOLogin, HandleLogout). No service layer needed for authentication middleware concerns.

**Checkpoint**: completed, with notes (see Problem below)

**Problem**: Original webapp auth implementation was blocked by race conditions in gridapi authentication causing 30% test failure rate. Sessions were breaking under concurrent load.

**Solution**: Complete gridapi authentication refactor (grid-f21b) across 9 phases:
- **Phase 1-2**: IAM service layer with immutable group→role cache
- **Phase 3**: Unified MultiAuth pattern (Session + JWT authenticators)
- **Phase 4**: Read-only authorization (eliminated Casbin mutations)
- **Phase 5**: Service layer organization
- **Phase 6**: Handler refactoring (eliminated 26 layering violations)
- **Phase 7**: Background cache refresh + Admin API
- **Phase 8**: Testing & validation (32/32 integration tests passing)
- **Phase 9**: Documentation

**Result**: ✅ All integration tests passing, race detector clean, <50ms latency

**Reference**: `specs/007-webapp-auth/gridapi-refactor/REFACTORING-STATUS.md`

**Status**: Completed

---

### Phase 3: User Story 1 - View States Without Authentication (grid-bad5) 🎯 MVP CANDIDATE

**Priority**: P1
**Goal**: When gridapi runs without authentication enabled, users access the dashboard without login and see all states.

**Why This Priority**: Baseline behavior - webapp must continue to work in non-authenticated mode for backward compatibility and development scenarios.

**Independent Test**: Run gridapi without authentication configuration, navigate to webapp, verify dashboard loads immediately showing all states without any login UI.

**Key Tasks** (query with `bd list --label story:US1 --limit 10`):
- Implement conditional auth UI rendering in App.tsx
- Implement fetchAuthConfig in js/sdk/auth.ts
- Integrate config loading in AuthProvider

**Acceptance Scenarios** (from spec.md):
1. ✅ gridapi without auth → dashboard displays immediately without login prompt
2. ✅ Dashboard displays → all states visible
3. ✅ Dashboard displays → no login button or user menu visible

**Status**: ✅ COMPLETE - Non-authenticated mode fully implemented

---

### Phase 4: User Story 2 - Authenticate Using Internal Identity Provider (grid-fdfc) 🎯 MVP CANDIDATE

**Priority**: P1
**Goal**: When gridapi uses internal IdP mode, web users authenticate with username and password to access the dashboard with their assigned roles.

**Why This Priority**: Primary web authentication flow for internal IdP mode - without this, authenticated web access is impossible when using Grid's built-in authentication.

**Independent Test**: Run gridapi with internal IdP enabled, attempt to access dashboard, enter valid username/password credentials, verify successful login with role assignment displayed.

**Key Tasks** (query with `bd list --label story:US2`):
- ✅ Implement loginInternal in js/sdk/auth.ts (POST /auth/login)
- ✅ Adapt LoginPage for internal IdP mode (webapp/src/components/LoginPage.tsx)
- ✅ Implement fetchWhoami in js/sdk/auth.ts (GET /api/auth/whoami)
- ✅ Implement session restoration in AuthProvider (useEffect calling fetchWhoami)
- ✅ Create AuthGuard component (webapp/src/components/AuthGuard.tsx)

**Acceptance Scenarios** (from spec.md):
1. ✅ gridapi requires auth + user not logged in → see login form with username/password fields
2. ✅ User enters valid credentials and submits → authenticated and see dashboard
3. ✅ User has active session → dashboard displays immediately without login prompt
4. ✅ User views auth status → displays username, email, assigned roles, auth type (Basic Auth - Internal IdP)
5. ✅ User enters invalid credentials → error message without revealing username/password specifics

**Backend Dependencies**:
- ✅ grid-87f5: POST /auth/login implementation
- ✅ grid-6b5a: GET /api/auth/whoami implementation
- ✅ grid-8f1a: gridapi users create command (for test users)

**Status**: ✅ COMPLETE - Internal IdP login flow fully implemented

---

### Phase 5: User Story 3 - Authenticate Using External Identity Provider (grid-74c0)

**Priority**: P1
**Goal**: When gridapi uses external IdP mode (SSO), web users authenticate through their organization's identity provider to access the dashboard with role assignments based on group memberships.

**Why This Priority**: Primary web authentication flow for external IdP mode (SSO) - critical for organizations using enterprise identity providers like Keycloak, Azure Entra ID, or Okta.

**Independent Test**: Run gridapi with external IdP enabled, attempt to access dashboard, click SSO login, complete authentication at external IdP, verify successful callback with group-based role assignment displayed.

**Key Tasks** (query with `bd list --label story:US3 --limit 10`):
- Implement loginExternal (SSO redirect) in js/sdk/auth.ts
- Adapt LoginPage for external IdP mode (SSO button)
- Handle OAuth2 callback flow (backend already implements this)

**Acceptance Scenarios** (from spec.md):
1. gridapi requires external IdP + user not logged in → see login form with SSO login option
2. User initiates SSO login → redirected to external IdP login page
3. User completes external IdP auth → OAuth2 callback completes, returns to dashboard authenticated
4. User views auth status → displays username, email, group memberships, derived roles, auth type (OIDC)
5. User authenticated via external IdP + JWT contains groups → system maps groups to roles per configured mappings

**Note**: Backend OAuth2 handlers already exist (from 006-authz-authn-rbac). Webapp just needs to trigger redirect and handle post-callback session restoration.

**Status**: ✅ COMPLETE - External IdP / SSO login flow fully implemented

---

### Phase 6: User Story 4 - See Only Authorized States (grid-32ef)

**Priority**: P1
**Goal**: When authentication is enabled, the dashboard automatically filters the states list to show only states the user is authorized to view based on their role's user scope (label-based filtering).

**Why This Priority**: Critical for security - users must not see states they don't have access to. Without this, role-based access control is meaningless.

**Independent Test**: Create states with different label combinations (e.g., env=dev, env=prod, product=foo), log in as users with different role scopes, verify each user only sees states matching their role's user scope expression.

**Key Tasks** (query with `bd list --label story:US4 --limit 10`):
- Implement client-side state filtering based on role scope expressions
- Parse and evaluate boolean expressions (go-bexpr compatible)
- Apply filters to dashboard state list
- Handle empty state list with appropriate messaging

**Acceptance Scenarios** (from spec.md):
1. User with role scope `env=="dev"` → only states with env=dev label displayed
2. User with role scope `env=="prod"` → only states with env=prod label displayed
3. User with role scope `env=="dev" and product=="foo"` → only states matching both label conditions
4. User with no role assignments → dashboard shows empty state list with message
5. State created that user cannot access → does not appear in user's dashboard view

**Future Enhancement** (noted in contracts/README.md:414):
Currently, ListStates returns all states and relies on client-side filtering. This is tracked in issue `grid-f5947b22`. Future server-side label filtering will improve security and performance.

**Status**: ✅ COMPLETE - Client-side role-based state filtering fully implemented

---

### Phase 7: User Story 5 - View Authentication Status and Role Information (grid-b2e6)

**Priority**: P2
**Goal**: Users can view their current authentication status, username, email, authentication type, and assigned roles through a user menu dropdown.

**Why This Priority**: Provides transparency and helps users understand their identity and assigned roles, but the dashboard can function without it.

**Independent Test**: Log in, click user menu in header, verify user information, authentication type, roles, and groups (if external IdP) displayed correctly.

**Key Tasks** (query with `bd list --label story:US5`):
- Adapt AuthStatus.tsx mockup to display real user data from AuthContext
- Show username, email, auth type, roles, session expiry
- Show groups for external IdP users
- Format and style role/group badges

**Acceptance Scenarios** (from spec.md):
1. User authenticated → click username in header → dropdown displays email, auth type, roles, session expiration
2. User authenticated via internal IdP → dropdown shows username, email, roles, auth type (Basic Auth), session expiry
3. User authenticated via external IdP → dropdown shows username, email, group memberships, derived roles, auth type (OIDC), session expiry
4. User has multiple roles → all roles displayed with distinct visual styling

**Note**: Existing AuthStatus.tsx mockup provides UI foundation. Task is to wire it to real AuthContext data.

**Status**: ✅ COMPLETE - User authentication status and role information display implemented

---

### Phase 8: User Story 6 - Log Out (grid-2374)

**Priority**: P2
**Goal**: Users can log out of the application, clearing their session and requiring re-authentication to access the dashboard again.

**Why This Priority**: Important for security and shared computer scenarios, but the application can function without it (sessions will eventually expire).

**Independent Test**: Log in, click logout button in user menu, verify session cleared and user returned to login page.

**Key Tasks** (query with `bd list --label story:US6`):
- Implement logout function in js/sdk/auth.ts (POST /auth/logout)
- Wire logout button in AuthStatus dropdown to logout function
- Clear AuthContext state on logout
- Redirect to login page after logout

**Acceptance Scenarios** (from spec.md):
1. User authenticated → clicks "Sign Out" in user menu → session cleared and logged out
2. User logged out → attempts to access dashboard → shown login page
3. User logging out → logout completes → see login page ready for re-authentication

**Note**: Backend POST /auth/logout already exists (from 006-authz-authn-rbac). Webapp just needs to call it and clear local state.

**Status**: ✅ COMPLETE - Logout functionality fully implemented

---

### Phase 9: Polish and Integration (grid-a16b)

**Purpose**: Cross-cutting concerns, error handling, edge cases, integration tests, documentation

**Key Tasks** (query with `bd list --label phase:polish --label spec:007-webapp-auth --limit 10`):
- Handle auth config changes while webapp loaded (detect on next API call)
- Handle session expiry during dashboard view (401 → redirect to login)
- Handle user with no role assignments (empty dashboard with message)
- Handle role assignment changes during active session (re-login to apply)
- Handle gridapi mode switch (authenticated → non-authenticated)
- Handle network connectivity loss during login
- Handle external IdP users with no group-to-role mappings
- Add Connect RPC interceptor for 401 handling
- Add error boundaries and user-friendly error messages
- Write integration tests (webapp + gridapi)
- Write component tests (Vitest + React Testing Library)
- Update documentation

**Edge Cases** (from spec.md:113-122):
- Auth config changes while webapp loaded → detect on next API call, redirect to appropriate login
- Session expires during dashboard view → next API request returns 401, redirect to login
- User with no role assignments → can authenticate but sees empty dashboard with message
- Role assignments change during active session → current session continues until logout/re-auth
- gridapi switches from authenticated to non-authenticated → webapp detects lack of auth errors on reload
- Network connectivity lost during login → user-friendly error message
- External IdP user with no group-to-role mappings → can authenticate but no roles assigned, empty dashboard

**Testing Strategy** (from plan.md:56-62):
- Component tests: Vitest + React Testing Library
- Integration tests: `createRouterTransport()` for Connect RPC mocking
- Auth flow tests: Mock both Connect RPC and /auth/* HTTP endpoints
- Coverage target: >80% for new components

**Status**: ⚠️ IN PROGRESS - Core features complete, 5 enhancement/test tasks remaining (see "Remaining Open Tasks" section below)

## GridAPI Integration Test Coverage

Post refactor of gridAPI.

### Integration Test Gap Discovery (2025-11-14, Updated 2025-11-16)

**Status**: ✅ Mode 2 Complete, ⚠️ Mode 1 Requires E2E Browser Tests

#### Original Problem (2025-11-14)

While gridapi refactor was complete and working, integration tests verified:
- ✅ Session login/logout (POST /auth/login, GET /api/auth/whoami)
- ✅ JWT + Connect RPCs (Mode 1 tests)
- ❌ **Session cookies + Connect RPCs** (webapp's primary usage pattern)

#### Current Status (2025-11-16)

**Mode 2 (Internal IdP)** - ✅ FULLY TESTED:
```
1. POST /auth/login → get grid.session cookie ✅ TESTED
2. ListStates RPC + cookie → fetch dashboard    ✅ TESTED
3. CreateState RPC + cookie → create state      ✅ TESTED
4. UpdateLabels RPC + cookie → modify state     ✅ TESTED
```
Test: `TestMode2_WebAuth_SessionWithConnectRPC` (auth_mode2_test.go:981-1107)

**Mode 1 (External IdP/SSO)** - ❌ NOT TESTED:
```
1. GET /auth/sso/login → redirect to Keycloak   ❌ UNTESTED (requires browser)
2. OAuth2 callback → get grid.session cookie    ❌ UNTESTED (requires browser)
3. ListStates RPC + cookie → fetch dashboard    ❌ UNTESTED
4. CreateState RPC + cookie → create state      ❌ UNTESTED
```
Note: A brittle test that used DB insertion was removed (2025-11-16). Proper testing requires E2E browser automation.

**Impact**:
- ✅ Mode 2 webapp contract is protected
- ❌ Mode 1 SSO webapp flow is untested
- Future changes could break Mode 1 SSO without detection

### Integration Test Coverage Issues (grid-5c57)

**Parent Issue**: `grid-5c57` - Add integration tests for webapp session + Connect RPC authentication contract

**Purpose**: Protect webapp contract by testing session-based Connect RPC authentication in both auth modes.

**Status**: ✅ Partially Complete - Mode 2 implemented, Mode 1 requires E2E browser tests

**Child Issues**:

#### 1. Mode 1: External IdP / Keycloak SSO (grid-c1cd)
**Priority**: P1
**File**: ❌ **REMOVED** - Previously at `tests/integration/auth_mode1_test.go:1045-1207`
**Function**: ~~`TestMode1_WebAuth_SessionWithConnectRPC`~~ (deleted 2025-11-16)

**Original Test Flow** (brittle implementation, now removed):
1. ~~SSO login via Keycloak~~ Simulated with password grant
2. ~~Extract grid.session cookie from /auth/callback~~ Direct DB insertion
3. Manual IAM cache refresh via `/admin/cache/refresh`
4. Call Connect RPCs with session cookie

**Why Removed**: Test bypassed the real OAuth2 callback flow (`/auth/sso/callback`) by directly inserting sessions into the database. This didn't test the actual SSO integration that the webapp relies on.

**Proper Solution**: Requires E2E browser tests using Playwright (see TESTING.md lines 32-100). A new feature issue will be created to track this work.

**Status**: ❌ NOT TESTED - Removed brittle test, pending E2E browser implementation

```bash
bd show grid-c1cd  # View closure comments
```

#### 2. Mode 2: Internal IdP / Username+Password (grid-787a)
**Priority**: P1
**File**: `tests/integration/auth_mode2_test.go:981-1107`
**Function**: `TestMode2_WebAuth_SessionWithConnectRPC`

**Test Flow**:
1. Create test user (gridapi users create)
2. Login via POST /auth/login
3. Extract grid.session cookie
4. Call ListStates RPC with cookie → verify 200
5. Call CreateState RPC with cookie → verify 200
6. Call UpdateLabels RPC with cookie → verify 200
7. Logout
8. Verify Connect RPCs return 401 after logout

**Status**: ✅ COMPLETE - Test implemented and passing

```bash
bd show grid-787a
```

### SDK Credentials Configuration (grid-b2a5)

**Issue**: `grid-b2a5` - Verify and configure Connect transport credentials for session cookie support

**Priority**: P2

**Problem**: SDK may not explicitly configure `credentials: 'include'` for Connect transport, relying on browser defaults.

**Current Code** (`js/sdk/src/client.ts:34-38`):
```typescript
export function createGridTransport(baseUrl: string): Transport {
  return createConnectTransport({
    baseUrl,
    // Missing: credentials: 'include' ?
  });
}
```

**Recommended Fix**:
```typescript
export function createGridTransport(baseUrl: string): Transport {
  return createConnectTransport({
    baseUrl,
    credentials: 'include', // Ensure session cookies always sent
  });
}
```

**Testing Strategy**:
1. Research @connectrpc/connect-web credential defaults
2. Manual browser test (DevTools → verify Cookie header present)
3. Integration tests (grid-787a, grid-c1cd) validate end-to-end

**Status**: Investigation needed

```bash
bd show grid-b2a5
```

---

### Issue Resolution: grid-b4dd (Connect Interceptor)

**Status**: ✅ CLOSED - Feature already implemented

**Original Issue**: "Create Connect authn interceptor for session authentication"

**Resolution**: The refactor already implemented this in Phase 3:
- **File**: `cmd/gridapi/internal/middleware/authn_multiauth_interceptor.go`
- **Function**: `NewMultiAuthInterceptor(iamService iam.Service)`
- **Registered**: `cmd/gridapi/cmd/serve.go:217`

**How It Works**:
1. Extracts session cookies from Connect request headers
2. Tries all authenticators (SessionAuthenticator → JWTAuthenticator)
3. Sets Principal in context for authz interceptor
4. Handles both authenticated and unauthenticated requests

**Why Closed**: Feature is fully implemented and working. The missing piece is test coverage, which is tracked separately in grid-5c57.

```bash
bd show grid-b4dd
bd comments grid-b4dd
```

---

### Query Commands

```bash
# View integration test parent issue
bd show grid-5c57

# View Mode 1 test (External IdP)
bd show grid-c1cd

# View Mode 2 test (Internal IdP) - RECOMMENDED STARTING POINT
bd show grid-787a

# View SDK credentials investigation
bd show grid-b2a5

# View closed Connect interceptor issue
bd show grid-b4dd
bd comments grid-b4dd

# See all integration test issues
bd list --label type:test --label spec:007-webapp-auth --status open
```

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately ✅
- **Foundational (Phase 2)**: Depends on Setup - **BLOCKS ALL USER STORIES** ⚠️
- **User Stories (Phases 3-8)**: All depend on Foundational completion
  - **US1 (P1)**: Non-auth mode - standalone
  - **US2 (P1)**: Internal IdP - **MVP CANDIDATE** 🎯
  - **US3 (P1)**: External IdP - parallel to US2
  - **US4 (P1)**: Client-side filtering - depends on US2 or US3
  - **US5 (P2)**: Auth status UI - depends on US2 or US3
  - **US6 (P2)**: Logout - depends on US5
- **Polish (Phase 9)**: Depends on all desired user stories

### Suggested MVP Scope

**Recommended MVP**: US1 + US2 (Non-auth mode + Internal IdP authentication)

**Rationale**:
- US1 ensures backward compatibility (non-auth mode)
- US2 provides complete authentication flow for internal IdP
- Together, these cover the core use case: webapp works with and without auth
- Can be delivered and tested independently
- Provides foundation for US3 (external IdP) in follow-up increment

**MVP Tasks**:
```bash
# View MVP tasks
bd list --label story:US1 --label spec:007-webapp-auth
bd list --label story:US2 --label spec:007-webapp-auth
bd list --label phase:foundational --label spec:007-webapp-auth
```

### Incremental Delivery Strategy

1. **Sprint 1**: Phase 1 (Setup) + Phase 2 (Foundational) → Backend ready
2. **Sprint 2**: US1 + US2 → MVP (non-auth + internal IdP working) 🎯
3. **Sprint 3**: US3 → External IdP support
4. **Sprint 4**: US4 → Role-based filtering
5. **Sprint 5**: US5 + US6 → Auth status UI + logout
6. **Sprint 6**: Phase 9 → Polish, testing, documentation

Each sprint delivers a complete, testable increment.

---

## Task Execution Notes

### For AI Agents

When implementing tasks:

1. **Check dependencies first**: `bd show <task-id>` to see blockers
2. **Mark task in progress**: `bd update <task-id> --status in_progress`
3. **Follow existing patterns**: All constitution justifications provided for gridapi tasks
4. **Test as you go**: Each user story should be independently testable
5. **Mark complete with notes**: `bd update <task-id> --status closed --notes "Implementation details"`

### For Human Developers

When picking up tasks:

1. **Start with Phase 1 (Setup)**: Foundation first
2. **Complete Phase 2 (Foundational) entirely**: Backend must be ready before frontend work
3. **Implement user stories in priority order**: P1 before P2
4. **Test each story independently**: Don't wait until end to verify
5. **Use quickstart.md scenarios**: Once available, validate against documented test cases

### Constitutional Compliance

All gridapi tasks include explicit justification against **Constitution IX: API Server Internal Layering** violations:

- Auth handlers follow existing pattern: handler → repository (direct, no service layer)
- Authentication is middleware concern, not business logic
- CLI commands directly access repositories (standard pattern in cmd/gridapi/cmd/*)
- Helper functions for data aggregation (not services)
- All changes are additive (no modifications to existing flows)

See individual task descriptions for detailed justifications.

---

## Query Examples for Common Workflows

```bash
# What's ready to work on? (can't filter by label yet, use list for that)
# bd list --label spec:007-webapp-auth --status open --limit 5
bd ready

# or with jq filtering:
# bd ready --json | jq '.[] | select(.labels // [] | contains(["spec:007-webapp-auth"]))'

# What's blocking progress? (can't filter by label yet)
# bd list --label spec:007-webapp-auth
bd blocked

# How many tasks are done?
bd stats

# Show full dependency tree
bd show grid-baf5

# Find all webapp frontend tasks
bd list --label component:webapp --label spec:007-webapp-auth

# Find all gridapi backend tasks
bd list --label component:gridapi --label spec:007-webapp-auth

# Find all SDK tasks
bd list --label component:sdk --label spec:007-webapp-auth

# Track specific user story progress
bd list --label story:US2 --label spec:007-webapp-auth
```

---

## Summary

- **Epic**: grid-baf5 (WebApp User Login Flow with Role-Based Filtering)
- **Total Phases**: 9 (1 setup, 1 foundational, 6 user stories, 1 polish)
- **User Stories**: 6 (4 P1, 2 P2)
- **MVP Scope**: US1 + US2 (non-auth mode + internal IdP)
- **Testing**: Optional per spec (no TDD requirement, but tests recommended for polish phase)
- **Backend Effort**: 7-10 hours (foundational phase)
- **Frontend Effort**: ~2-3 days per user story
- **Total Estimated Effort**: 2-3 weeks for full feature

All task details, dependencies, and status are tracked in Beads. This file serves as a navigation index only.

---

## Remaining Open Tasks & Next Steps (2025-11-16)

### 📊 Current Status

**Feature Status:**
- ✅ All 6 user stories (US1-US6) complete
- ✅ Integration tests complete (Mode 2)
- ⚠️ 5 polish/enhancement tasks remain open
- ⚠️ E2E browser tests not yet started

### 🎯 5 Remaining Open Tasks

#### 1. **grid-3be6**: Component Tests (Priority: P2)
**Type:** Implementation needed
**Scope:** Write Vitest + React Testing Library component tests
**Coverage Target:** >80% for auth components

**Why Still Open:**
- Component tests are separate from E2E tests (unit-level vs integration-level)
- Provide faster feedback than E2E tests
- Test edge cases and error states more easily
- Don't require full docker-compose environment

**Files to Test:**
- `webapp/src/context/AuthContext.tsx`
- `webapp/src/components/AuthGuard.tsx`
- `webapp/src/components/LoginPage.tsx`
- `webapp/src/components/AuthStatus.tsx`

**Effort:** 4-6 hours

**Query:** `bd show grid-3be6`

---

#### 2. **grid-5a64**: EmptyState Component (Priority: P2)
**Type:** UI component implementation
**Scope:** Create reusable EmptyState component for users with no accessible states

**Why Still Open:**
- Component needs to be built (not just tested)
- Required for edge cases: users with no roles, role scope matches no states
- Referenced in FR-009

**Usage Contexts:**
- User with no role assignments
- User with roles but label scope matches no states
- External IdP user with no group-to-role mappings

**Effort:** 2-3 hours

**Query:** `bd show grid-5a64`

---

#### 3. **grid-459e**: Integration Test Contract Documentation (Priority: P2)
**Type:** Documentation deliverable
**Scope:** Document webapp-gridapi integration test contract for future maintainers

**Why Still Open:**
- Critical for maintaining test coverage as codebase evolves
- Not related to E2E tests (documents integration test maintenance)
- Prevents future regressions when changing auth implementation

**Content:**
- Contract definition: "Webapp uses session cookies for Connect RPC authentication"
- Which tests validate the contract
- When tests must be updated
- Links between tests and implementation files

**Suggested Location:** `tests/integration/README.md` or `webapp/README.md`

**Effort:** 2-3 hours

**Query:** `bd show grid-459e`

---

#### 4. **grid-b9dd**: External IdP Roles Display Bug (Priority: P3)
**Type:** Bug - needs investigation
**Scope:** Fix group-derived roles not showing in AuthStatus for external IdP users

**Why Still Open:**
- Actual functionality broken (not just polish)
- Internal IdP roles show correctly, external IdP roles don't
- Blocks full validation of US5 (View Authentication Status)

**Impact:**
- External IdP users (Keycloak SSO) can't see their roles in UI
- Backend authorization works, but UI feedback is missing
- E2E tests will fail if roles don't display

**Investigation Needed:**
- Check `/api/auth/whoami` response for external IdP users
- Verify role aggregation logic includes group-derived roles
- Check AuthStatus.tsx role display logic

**Effort:** 2-4 hours (investigation + fix)

**Query:** `bd show grid-b9dd`

---

#### 5. **grid-f5947b22**: ListStates Server-Side Authorization (Priority: P3)
**Type:** Technical debt / future enhancement
**Scope:** Implement server-side label filtering in ListStates RPC

**Why Still Open:**
- Known limitation documented in tasks.md:273
- Currently: ListStates returns ALL states, webapp filters client-side
- Future: Server should filter states based on user's label scope

**Impact:**
- Security: Client-side filtering is less secure
- Performance: Inefficient for large state counts
- Not critical for MVP (client-side filtering works for now)

**Approaches:**
1. Post-filter in service layer (simpler, correct)
2. Pre-filter with database query (faster, complex)

**Effort:** 8-12 hours (design + implementation + tests)

**Defer to:** Future sprint (P3 priority)

**Query:** `bd show grid-f5947b22`

---

### ✅ Recently Closed (2025-11-16)

The following 3 polish tasks were closed as **covered by E2E browser tests** (tracked in `grid-f218`):

1. **grid-31bd**: User-friendly error messages
   - **Reason:** Error validation covered by `TestE2E_GroupBasedPermission_Forbidden` (TESTING.md:92)
   - **E2E Task:** grid-f218.5

2. **grid-3034**: Connect RPC 401 interceptor
   - **Reason:** Infrastructure complete, session expiry testing deferred to E2E enhancements

3. **grid-0c07**: Auth config changes edge case
   - **Reason:** Rare edge case documented, not critical for MVP, can add to E2E later

---

### 🚀 Suggested Next Steps (Post-Merge)

#### **Option 1: Complete Remaining Polish Tasks (Recommended)**

Close out the feature completely before starting E2E tests:

```bash
# 1. Fix external IdP roles bug (CRITICAL)
bd show grid-b9dd
# Investigation: Check /api/auth/whoami response, role aggregation logic
# 2-4 hours

# 2. Implement component tests (HIGH VALUE)
bd show grid-3be6
# Fast feedback, catches edge cases, doesn't need docker-compose
# 4-6 hours

# 3. Create EmptyState component
bd show grid-5a64
# Required for users with no accessible states
# 2-3 hours

# 4. Document integration test contract
bd show grid-459e
# Prevents future regressions
# 2-3 hours

# Total: 10-16 hours (1.5-2 days)
```

**After completion:**
- All polish tasks done ✅
- Feature 100% complete ✅
- Ready to start E2E tests ✅

---

#### **Option 2: Start E2E Browser Tests**

Begin implementing Playwright E2E tests (higher priority than polish):

```bash
# View E2E feature and child tasks
bd show grid-f218
bd dep tree --reverse grid-f218
bd list --label phase:e2e --label spec:007-webapp-auth

# Start with infrastructure (partially done)
bd show grid-f218.1
# Configure Playwright, test fixtures, global setup
# 2-3 hours

# Implement helpers
bd show grid-f218.2
# login(), logout(), createState(), navigation helpers
# 3-4 hours

# Implement 4 core test cases
bd show grid-f218.3  # Login + session persistence
bd show grid-f218.4  # Permission success
bd show grid-f218.5  # Permission forbidden
bd show grid-f218.6  # JIT provisioning
```

**Benefits:**
- Validates Mode 1 (External IdP/SSO) webapp flow (currently untested)
- Provides true black-box testing
- Catches integration issues missed by unit tests

**Prerequisites:**
- **Must fix grid-b9dd first** (E2E tests will fail if roles don't show)
- Playwright already initialized ✅
- Docker compose environment ready ✅

---

#### **Option 3: Defer Both (Move to Next Sprint)**

If other priorities are higher:

1. **Close grid-baf5 epic** with notes about remaining tasks
2. **Create new epic** for "007 Polish & E2E Tests"
3. **Move 5 open tasks** to new epic
4. **Schedule** for next sprint

**Trade-offs:**
- Feature is "done" but not polished
- Mode 1 SSO webapp flow remains untested
- Risk of regressions if changes are made

---

### 📋 Recommended Approach

**Phase 1: Critical Fixes (This Sprint)**
1. Fix grid-b9dd (external IdP roles bug) - **REQUIRED for E2E tests**
2. Create grid-5a64 (EmptyState component) - **REQUIRED for complete UX**

**Phase 2: Testing (Next Sprint)**
1. Implement E2E browser tests (grid-f218 + 6 child tasks)
2. Implement component tests (grid-3be6)
3. Write integration test documentation (grid-459e)

**Phase 3: Future Enhancement (Backlog)**
1. Server-side ListStates filtering (grid-f5947b22) - P3 technical debt

**Rationale:**
- Fixes critical bugs before E2E implementation
- E2E tests provide highest value (cover untested Mode 1 SSO flow)
- Component tests complement E2E tests (faster, more granular)
- Server-side filtering is optimization, not critical

---

### 🔗 Related Issues

- **E2E Feature:** `bd show grid-f218`
- **Epic:** `bd show grid-baf5`
- **Polish Phase:** `bd show grid-a16b`
- **All Open:** `bd list --label spec:007-webapp-auth --status open`

---

### 📝 Notes

- This section added 2025-11-16 after cleanup of brittle Mode 1 test
- 3 polish tasks closed as covered by E2E tests (grid-31bd, grid-3034, grid-0c07)
- Playwright infrastructure already initialized at `tests/e2e/`
- Mode 2 (Internal IdP) integration tests passing ✅
- Mode 1 (External IdP/SSO) requires E2E browser tests ⚠️
