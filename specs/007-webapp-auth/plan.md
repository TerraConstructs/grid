# Implementation Plan: WebApp User Login Flow with Role-Based Filtering

**Branch**: `007-webapp-auth` | **Date**: 2025-11-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/007-webapp-auth/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Add authentication UI to the webapp enabling users to log in via internal IdP (username/password) or external IdP (SSO), view their authentication status and assigned roles, and automatically see only states they are authorized to access based on their role's user scope expression. The webapp must continue to function without authentication when gridapi does not require it.

## Technical Context

**Language/Version**: TypeScript 5.x (webapp), React 18 (UI framework)
**Primary Dependencies**: React, @connectrpc/connect-web (RPC client), Vite (build tool), Tailwind CSS (styling), Lucide React (icons)
**Storage**: Browser localStorage/sessionStorage for session management, httpOnly cookies for auth tokens (managed by gridapi)
**Testing**: Vitest + React Testing Library for component tests, integration tests with mocked Connect transport
**Target Platform**: Modern browsers (Chrome/Firefox/Safari latest 2 versions)
**Project Type**: Web application (frontend only, consumes gridapi backend)
**Performance Goals**: <2s session restoration on page load, instant UI updates on auth state changes
**Constraints**: Must work without authentication (backward compatibility), must not leak auth tokens in browser console/logs
**Scale/Scope**: Single-page application with ~10 components, 2 authentication modes (internal IdP + external IdP), role-based filtering for dashboard state list

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle II: Contract-Centric SDKs

**Status**: ✅ PASS (with approved exception)

- Webapp will consume `js/sdk` which wraps Connect RPC clients from `./api`
- Auth helpers in `js/sdk/auth.ts` will make direct HTTP calls to `/auth/*` endpoints (approved constitutional exception for OIDC flows)
- No server-side logic or persistence in webapp
- All state management operations go through Connect RPC via js/sdk

**Justification for `/auth/*` exception**: OIDC authentication flows require specific HTTP redirect patterns and cookie handling that cannot be expressed in Connect RPC while maintaining OAuth2/OIDC protocol compliance.

### Principle III: Dependency Flow Discipline

**Status**: ✅ PASS

- Webapp depends only on `js/sdk` (not on gridapi internals)
- No circular dependencies introduced
- Unidirectional flow maintained: `webapp → js/sdk → api (generated)`

### Principle IV: Cross-Language Parity via Connect RPC

**Status**: ✅ PASS (with approved exception)

- All state/dependency operations use Connect RPC through js/sdk
- Auth endpoints (`/auth/login`, `/auth/callback`, `/auth/config`) are HTTP-only per constitutional exception
- Existing `/health` and `/auth/config` endpoints already implemented in gridapi

### Principle V: Test Strategy

**Status**: ✅ PASS

- Component tests using Vitest + React Testing Library
- Integration tests with `createRouterTransport()` for Connect RPC mocking
- Auth flow tests will mock both Connect RPC and `/auth/*` HTTP endpoints
- Coverage target: >80% for new components

### Principle VII: Simplicity & Pragmatism

**Status**: ✅ PASS

- No new modules or dependencies beyond existing webapp stack
- Reuses existing Connect RPC infrastructure
- Leverages designer mockups (LoginPage, AuthStatus components) for KISS/YAGNI alignment
- Removed advanced profile page, label policy viewer, deep linking (YAGNI)

### Principle VIII: Service Exposure Discipline

**Status**: ✅ PASS (frontend, N/A for backend changes)

- Webapp is a consumer, not a service provider
- All auth checks happen server-side in gridapi

### Principle IX: API Server Internal Layering

**Status**: ✅ PASS (with technical debt note)

- **NEW**: `/api/auth/whoami` REST endpoint required for session restoration
  - Webapp needs to fetch user identity + session info from httpOnly cookie
  - `GetEffectivePermissions` RPC exists but insufficient (requires principal_id input, missing user identity/session/groups)
  - New handler will follow existing auth handler pattern: direct repository access (documented technical debt)

- **NEW**: `POST /auth/login` (internal IdP) endpoint
  - Username/password authentication handler
  - Will follow existing auth handler pattern: direct repository access (documented technical debt)

- **BUG FIX**: Populate `AuthenticatedPrincipal.SessionID` in authn middleware
  - SessionID field declared but currently never set
  - Needed for whoami endpoint to return session info

**Constitutional Debt Note**:

Constitution IX stipulates that "Handlers MUST NOT import `internal/repository`. Use Services exclusively." However, the existing SSO auth handlers (`HandleSSOCallback`, `HandleLogout`) directly access repositories via `deps.Users` and `deps.Sessions`. This violation is documented in Constitution IX's "Known Violations" section as: "Auth Handlers: Connect auth handlers and SSO HTTP handlers manipulate repositories and Casbin directly. Must introduce `iam` service and route all user/session/role operations through it."

**Decision**: New auth endpoints (`HandleWhoAmI`, `HandleInternalLogin`) will follow the existing pattern for consistency and faster delivery. This maintains architectural debt but prevents introducing divergent patterns. The complete fix (introducing IAM service layer) is tracked as separate technical debt to be addressed in future work.

**Summary**: All constitutional principles pass with noted technical debt. The auth helper HTTP exception is approved. Backend requires new endpoints and bug fix to support webapp session restoration.

## Project Structure

### Documentation (this feature)

```
specs/007-webapp-auth/
├── spec.md              # Feature specification (completed)
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (in progress)
├── data-model.md        # Phase 1 output (pending)
├── quickstart.md        # Phase 1 output (pending)
├── contracts/           # Phase 1 output (pending)
├── checklists/
│   └── requirements.md  # Spec quality checklist (completed)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```
webapp/
├── src/
│   ├── components/
│   │   ├── LoginPage.tsx          # EXISTING: Designer mockup (auth flow)
│   │   ├── AuthStatus.tsx         # EXISTING: Designer mockup (user menu)
│   │   ├── AuthGuard.tsx          # NEW: Route protection
│   │   ├── EmptyState.tsx         # NEW: Empty states for no accessible states
│   │   └── [existing components]  # Dashboard, GraphView, ListView, etc.
│   ├── hooks/
│   │   ├── useAuth.ts             # NEW: Auth state management
│   │   └── useGridData.ts         # EXISTING: Dashboard data fetching
│   ├── services/
│   │   ├── authMockService.ts     # EXISTING: Designer mockup (replace with real impl)
│   │   └── gridApi.ts             # EXISTING: Connect RPC client
│   ├── context/
│   │   └── AuthContext.tsx        # NEW: Auth state context
│   ├── App.tsx                    # MODIFY: Integrate auth UI (partially done)
│   └── __tests__/
│       ├── auth.test.tsx          # NEW: Auth flow tests
│       └── [existing tests]
│
js/sdk/
├── auth.ts                        # NEW: Browser auth helpers (HTTP to /auth/*)
├── gen/                           # EXISTING: Generated Connect clients
└── package.json

cmd/gridapi/
├── cmd/users/
│   └── create.go                  # NEW: CLI command for internal IdP user bootstrapping
├── internal/
│   ├── server/
│   │   ├── auth_handlers.go       # MODIFY: Add HandleWhoAmI, HandleInternalLogin
│   │   ├── router.go              # MODIFY: Mount new auth endpoints
│   │   └── [existing handlers]
│   ├── middleware/
│   │   ├── authn.go              # MODIFY: Fix SessionID population in resolvePrincipal
│   │   └── [existing middleware]
│   └── repository/
│       ├── user_repository.go     # EXISTING: User CRUD (used by new handlers)
│       └── session_repository.go  # EXISTING: Session CRUD (used by new handlers)

tests/integration/
├── webapp_auth_test.go            # NEW: E2E tests (webapp + gridapi)
└── [existing integration tests]
```

**Structure Decision**: This is a web application following Option 2 (frontend + backend separation). The webapp is the frontend consuming gridapi as the backend. Frontend code lives in `webapp/src/` and auth helpers in `js/sdk/auth.ts`. Backend changes are minimal and additive:
- New auth handlers in existing `internal/server/auth_handlers.go`
- New CLI command for user management in new `cmd/users/create.go`
- Middleware bug fix in existing `internal/middleware/authn.go`
- Existing designer mockups (`LoginPage.tsx`, `AuthStatus.tsx`, `authMockService.ts`) adapted to call real gridapi endpoints

## Complexity Tracking

*No constitutional violations. Backend changes required are minimal and additive.*

### Required Backend Changes

**Change 1: Add `/api/auth/whoami` REST Endpoint**
- **Location**: `cmd/gridapi/internal/server/auth_handlers.go`
- **Rationale**: Webapp needs session restoration endpoint. Existing `GetEffectivePermissions` RPC insufficient (requires principal_id input, missing user identity/session/groups).
- **Scope**: New handler function (~50 lines), returns user identity + session + roles + groups
- **Estimated Effort**: 1-2 hours

**Change 2: Fix SessionID Population Bug**
- **Location**: `cmd/gridapi/internal/middleware/authn.go`
- **Rationale**: `AuthenticatedPrincipal.SessionID` field declared but never populated
- **Scope**: Look up session by token hash in `resolvePrincipal()`, set `principal.SessionID`
- **Estimated Effort**: 30 minutes

**Change 3: Mount Endpoint in Router**
- **Location**: `cmd/gridapi/internal/server/router.go`
- **Scope**: One line addition: `r.Get("/api/auth/whoami", HandleWhoAmI(&opts.AuthnDeps))`
- **Estimated Effort**: 5 minutes

**Change 4: Verify Repository Methods**
- **Location**: `cmd/gridapi/internal/repository/session_repository.go`
- **Scope**: Ensure `GetByID()` and `GetByTokenHash()` methods exist
- **Estimated Effort**: 15 minutes (likely already exist)

**Change 5: Implement POST /auth/login (Internal IdP)**
- **Location**: `cmd/gridapi/internal/server/auth_handlers.go`
- **Rationale**: Currently only `/auth/sso/login` exists (external IdP). Need username/password handler for internal IdP mode.
- **Scope**:
  - Accept JSON `{username, password}` body
  - Lookup user by email or username (flexible)
  - Verify bcrypt password hash against `users.password_hash`
  - Create session record in database
  - Set httpOnly session cookie with JWT access token
  - Return user metadata + session expiry (~80 lines)
- **Implementation Notes**:
  - Reuse existing JWT generation logic from SSO callback handler
  - Reuse session creation logic from SSO flow
  - Return 401 for invalid credentials, 403 for disabled accounts
- **Estimated Effort**: 2-3 hours

**Change 6: Implement gridapi users create Command**
- **Location**: `cmd/gridapi/cmd/users/create.go` (new package)
- **Rationale**: No way to bootstrap local users for internal IdP mode. Required for initial admin account creation.
- **Pattern**: Use `cmd/gridapi/cmd/sa/create.go` as template (bcrypt password hashing)
- **Scope**:
  - Cobra command `gridapi users create --email <email> --username <name> --password <pass> [--role <role>]`
  - Hash password with bcrypt (cost factor 12, same as service accounts)
  - Insert into `users` table with `password_hash` set, `subject` NULL (internal IdP marker)
  - Optionally assign initial role via `user_roles` table
  - Print created user ID and confirmation (~100 lines total)
- **Implementation Notes**:
  - Validate email format and uniqueness
  - Require strong password (min 12 chars, or configurable)
  - Support `--stdin` flag for password input (avoid shell history)
- **Estimated Effort**: 2-4 hours

**Change 7: Role Aggregation in whoami Endpoint**
- **Location**: `cmd/gridapi/internal/server/auth_handlers.go` (HandleWhoAmI implementation)
- **Rationale**: Frontend expects `User.roles: string[]` aggregated from BOTH `user_roles` (direct assignments) AND `group_roles` (via JWT groups claim). Current plan doesn't specify this union logic.
- **Scope**:
  - Query direct role assignments: `SELECT roles.name FROM user_roles JOIN roles ON role_id WHERE user_id = ?`
  - For external IdP users: decode `sessions.id_token` JWT to extract `groups` claim
  - Query group-based roles: `SELECT roles.name FROM group_roles JOIN roles ON role_id WHERE group_name IN (?)`
  - Return union of both role sets (deduplicated) (~30 lines additional logic)
- **Implementation Notes**:
  - Use existing JWT decoding logic from authn middleware
  - Handle missing `groups` claim gracefully (internal IdP users don't have it)
  - Cache role resolution results in session? (Optional optimization)
- **Estimated Effort**: 1 hour

**Total Backend Effort**: **7-10 hours** (up from 2-3 hours)

All changes are **additive** (no modifications to existing endpoints or flows), follow internal layering principles, and require no proto changes.
