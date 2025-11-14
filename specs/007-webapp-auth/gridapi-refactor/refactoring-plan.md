# Grid API Architectural Refactoring Plan: Proper Layering & Race Condition Fix

**Created**: 2025-11-12
**Status**: Approved
**Epic**: Race condition fix and proper layering for gridapi authentication/authorization

## Problem Statement

The Grid API suffers from a **critical race condition** in its authentication middleware caused by using Casbin's global policy state as per-request temporary storage for dynamic group-to-role mappings. This results in:

- Intermittent 403 authorization failures for external IdP users
- Write amplification (9 DB queries per authenticated request)
- Lock contention under high concurrency (casbinMutex bottleneck)
- 26 layering violations (handlers/middleware accessing repositories directly)

## Root Causes

1. **Casbin JIT policy mutation**: Every request mutates Casbin enforcer state (AddGroupingPolicy)
2. **AutoSave enabled**: Immediate DB writes inside mutex-protected sections
3. **Temporal separation**: Resolution (step 3) and enforcement (step 6) have no isolation
4. **No service layer**: Authentication logic scattered across 5 middleware files
5. **Shared mutable state**: Global Casbin enforcer modified by concurrent requests

## Solution Strategy

Adopt **ABAC (Attribute-Based Access Control)** pattern:
- Resolve roles ONCE at authentication time
- Store in request context (immutable)
- Evaluate against Casbin policies WITHOUT state mutation
- Use immutable cache with atomic.Value for group→role mappings

## Design Principles

1. **Proper Layering**: Handlers → Services → Repositories (strict enforcement)
2. **Immutable Caches**: Use `atomic.Value` with copy-on-write snapshots
3. **Authenticator Interface**: Multiple implementations (JWT, Session) producing unified `Principal`
4. **Read-Only Request Path**: No repository writes during request handling
5. **Out-of-Band Cache Refresh**: Background goroutine or admin API for reload
6. **Keep RBAC**: Maintain dynamic group→role mappings and customer-defined policies

---

## Phase 1: Create Services Layer Foundation

**Priority**: P0
**Effort**: 6-8 hours
**Risk**: Low

### Objectives

- Establish proper directory structure for service layer
- Define core interfaces (Authenticator, IAM Service)
- Define Principal struct (normalized authentication result)

### New Directory Structure

```
cmd/gridapi/internal/
├── services/                    # NEW: All domain services
│   ├── iam/                     # NEW: Identity & Access Management
│   │   ├── service.go           # IAM service interface + implementation
│   │   ├── authenticator.go     # Authenticator interface
│   │   ├── principal.go         # Principal struct definition
│   │   └── (to be added in later phases)
│   └── (existing services move here in Phase 5)
```

### Tasks

#### Task 1.1: Create iam package structure
- Create `cmd/gridapi/internal/services/iam/` directory
- Add package documentation

#### Task 1.2: Define Authenticator interface

**File**: `cmd/gridapi/internal/services/iam/authenticator.go`

```go
package iam

import (
    "context"
    "net/http"
)

// Authenticator extracts and validates credentials, returning a Principal.
// Implementations: JWTAuthenticator (bearer tokens), SessionAuthenticator (cookies)
type Authenticator interface {
    // Authenticate validates credentials and returns a Principal with resolved roles.
    // Returns (nil, nil) if credentials not present (not an error, try next authenticator).
    // Returns (nil, error) if credentials invalid (authentication failed).
    Authenticate(ctx context.Context, req AuthRequest) (*Principal, error)
}

// AuthRequest wraps HTTP request data for authenticator implementations
type AuthRequest struct {
    Headers http.Header
    Cookies []*http.Cookie
}
```

#### Task 1.3: Define Principal struct

**File**: `cmd/gridapi/internal/services/iam/principal.go`

```go
package iam

// Principal represents an authenticated identity with resolved roles.
// This struct is immutable after construction - roles are computed once.
type Principal struct {
    // Subject is the stable OIDC subject or client identifier (unprefixed)
    Subject string

    // PrincipalID is the Casbin-ready identifier (e.g., user:alice@example.com)
    PrincipalID string

    // InternalID references the backing database record (users.id or service_accounts.id)
    InternalID string

    // Email is optional, present for human users
    Email string

    // Name is optional display name
    Name string

    // SessionID references the active session (if authenticated via cookie)
    SessionID string

    // Groups lists the IdP groups this principal belongs to
    Groups []string

    // Roles lists resolved role names (computed from user_roles ∪ group_roles)
    // This is the SOURCE OF TRUTH for authorization - NOT Casbin state
    Roles []string

    // Type differentiates users and service accounts
    Type PrincipalType
}

type PrincipalType string

const (
    PrincipalTypeUser           PrincipalType = "user"
    PrincipalTypeServiceAccount PrincipalType = "service_account"
)
```

#### Task 1.4: Define IAM Service interface

**File**: `cmd/gridapi/internal/services/iam/service.go`

Core service interface with all IAM operations (authentication, authorization, session/user/role management).

### Deliverables

- [ ] `internal/services/iam/` directory created
- [ ] `authenticator.go` with Authenticator interface
- [ ] `principal.go` with Principal struct
- [ ] `service.go` with Service interface definition
- [ ] Package compiles successfully

---

## Phase 2: Implement Immutable Group→Role Cache

**Priority**: P0
**Effort**: 4-6 hours
**Risk**: Low

### Objectives

- Eliminate race condition by removing shared mutable state
- Use `atomic.Value` for lock-free reads
- Implement copy-on-write refresh pattern

### Architecture

```
GroupRoleCache
    ├── snapshot (atomic.Value) → *GroupRoleSnapshot
    ├── Get() → *GroupRoleSnapshot (lock-free read)
    ├── Refresh(ctx) → atomically swap new snapshot
    └── GetRolesForGroups(groups) → []string (pure function)

GroupRoleSnapshot (immutable)
    ├── Mappings map[string][]string (group → roles)
    ├── CreatedAt time.Time
    └── Version int
```

### Tasks

#### Task 2.1: Implement GroupRoleCache

**File**: `cmd/gridapi/internal/services/iam/group_role_cache.go`

Key methods:
- `NewGroupRoleCache()` - Initialize with initial load
- `Get()` - Lock-free read via atomic.Value.Load()
- `Refresh(ctx)` - Build new map, atomic swap
- `GetRolesForGroups(groups)` - Pure function

#### Task 2.2: Write GroupRoleCache unit tests

**File**: `cmd/gridapi/internal/services/iam/group_role_cache_test.go`

Test coverage:
- `TestGroupRoleCache_InitialLoad` - Verify initial snapshot
- `TestGroupRoleCache_Refresh` - Verify atomic swap
- `TestGroupRoleCache_GetRolesForGroups` - Verify role computation
- `TestGroupRoleCache_Concurrent` - 1000 readers + 1 writer (race detector)
- `TestGroupRoleCache_EmptyGroups` - Edge case handling

#### Task 2.3: Integrate cache into IAM service

**File**: `cmd/gridapi/internal/services/iam/service.go` (implementation)

Add cache to service struct, implement `ResolveRoles()` method.

### Deliverables

- [ ] `group_role_cache.go` implemented
- [ ] `group_role_cache_test.go` with 90%+ coverage
- [ ] Cache integrated into IAM service
- [ ] All tests pass including race detector

---

## Phase 3: Implement Authenticator Pattern

**Priority**: P0
**Effort**: 6-8 hours
**Risk**: Medium

### Objectives

- Normalize authentication paths (JWT and Session)
- Multiple authenticators, single Principal output
- Replace scattered middleware with unified MultiAuth

### Tasks

#### Task 3.1: Implement JWTAuthenticator

**File**: `cmd/gridapi/internal/services/iam/jwt_auth.go`

- Extract Bearer token from Authorization header
- Verify JWT signature (reuse existing verifier)
- Extract claims (sub, email, groups)
- Check JTI revocation
- Resolve user/service account
- Compute roles via cache
- Return Principal

#### Task 3.2: Implement SessionAuthenticator

**File**: `cmd/gridapi/internal/services/iam/session_auth.go`

- Extract session cookie
- Hash cookie value
- Lookup session in DB
- Validate session (not revoked, not expired)
- Lookup user
- Extract groups from stored ID token
- Compute roles via cache
- Return Principal

#### Task 3.3: Implement MultiAuth middleware

**File**: `cmd/gridapi/internal/middleware/authn_multiauth.go`

- Try authenticators in order (session first, then JWT)
- First success wins
- Store Principal in context
- Unified entry point for both HTTP and Connect

### Deliverables

- [ ] `jwt_auth.go` implemented
- [ ] `session_auth.go` implemented
- [ ] `authn_multiauth.go` implemented
- [ ] Unit tests for each authenticator
- [ ] Integration test showing both paths work

---

## Phase 4: Refactor Authorization to Use Principal.Roles

**Priority**: P0
**Effort**: 4-6 hours
**Risk**: Low

### Objectives

- Remove ALL Casbin state mutation
- Use Principal.Roles as source of truth
- Read-only Casbin policy evaluation
- Delete casbinMutex

### Tasks

#### Task 4.1: Implement read-only Casbin authorization

**File**: `cmd/gridapi/internal/services/iam/casbin_readonly.go`

Function: `AuthorizeWithRoles(enforcer, roles, obj, act, labels)`
- Iterate over principal's roles
- For each role, check if Casbin policy allows
- Return true if ANY role grants permission
- No AddGroupingPolicy, no state mutation

#### Task 4.2: Refactor authorization middleware

**Files**:
- `cmd/gridapi/internal/middleware/authz_http.go`
- `cmd/gridapi/internal/middleware/authz_connect.go`

Changes:
- Get Principal from context (set by MultiAuth)
- Use `iamService.Authorize(principal, obj, act, labels)`
- Remove all ApplyDynamicGroupings calls
- Remove LoadPolicy calls

#### Task 4.3: Remove Casbin mutation code

**Delete/Modify**:
- Remove `casbinMutex` from `authn_shared.go`
- Remove `ApplyDynamicGroupings()` from `auth/groups.go`
- Remove `clearUserGroupings()` function
- Update `cmd/gridapi/cmd/serve.go` - disable AutoSave

### Deliverables

- [ ] Read-only authorization implemented
- [ ] Authorization middleware refactored
- [ ] All Casbin mutation code removed
- [ ] casbinMutex deleted
- [ ] AutoSave disabled in serve.go

---

## Phase 5: Move Existing Services to services/

**Priority**: P1
**Effort**: 2-4 hours
**Risk**: Low

### Objectives

- Proper directory structure
- All services in `internal/services/`
- Clean import paths

### Tasks

#### Task 5.1: Move service packages

```bash
git mv cmd/gridapi/internal/state cmd/gridapi/internal/services/state
git mv cmd/gridapi/internal/dependency cmd/gridapi/internal/services/dependency
git mv cmd/gridapi/internal/graph cmd/gridapi/internal/services/graph
git mv cmd/gridapi/internal/tfstate cmd/gridapi/internal/services/tfstate
```

#### Task 5.2: Update import paths

Update all files importing these packages:
- Replace `internal/state` → `internal/services/state`
- Replace `internal/dependency` → `internal/services/dependency`
- Replace `internal/graph` → `internal/services/graph`
- Replace `internal/tfstate` → `internal/services/tfstate`

### Deliverables

- [ ] All services moved to `internal/services/`
- [ ] All imports updated
- [ ] Project compiles successfully
- [ ] Tests pass

---

## Phase 6: Refactor Handlers to Use IAM Service

**Priority**: P1
**Effort**: 4-6 hours
**Risk**: Medium

### Objectives

- Fix all layering violations
- Handlers use services, not repositories
- Delete auth_helpers.go

### Tasks

#### Task 6.1: Refactor auth handlers

**File**: `cmd/gridapi/internal/server/auth_handlers.go`

Change all repository calls to IAM service calls:
- `deps.Users.GetByEmail()` → `iamService.GetUserByEmail()`
- `deps.Sessions.Create()` → `iamService.CreateSession()`
- `deps.ServiceAccounts.Create()` → `iamService.CreateServiceAccount()`

#### Task 6.2: Delete auth_helpers.go

**Delete**: `cmd/gridapi/internal/server/auth_helpers.go`

Move logic to IAM service:
- `resolveEffectiveRoles()` → `iamService.ResolveRoles()`
- `buildGroupRoleMap()` → handled by GroupRoleCache

#### Task 6.3: Update handler dependencies

**File**: `cmd/gridapi/internal/server/deps.go` (create if needed)

Remove repository dependencies, keep only services:
```go
type Deps struct {
    IAMService        iam.Service
    StateService      *state.Service
    DependencyService *dependency.Service
}
```

### Deliverables

- [ ] Handlers refactored to use IAM service
- [ ] auth_helpers.go deleted
- [ ] Handler dependencies updated
- [ ] Zero handlers import `repository` package
- [ ] All tests pass

---

## Phase 7: Background Cache Refresh & Admin API

**Priority**: P2
**Effort**: 4-6 hours
**Risk**: Low

### Objectives

- Automatic cache refresh
- Manual refresh via admin API
- Operational flexibility

### Tasks

#### Task 7.1: Implement background refresh goroutine

**File**: `cmd/gridapi/cmd/serve.go`

- Start goroutine on server start
- Ticker: every 5 minutes (configurable)
- Call `iamService.RefreshGroupRoleCache()`
- Log success/failure

#### Task 7.2: Create admin API for manual refresh

**File**: `cmd/gridapi/internal/server/admin_handlers.go` (new)

Endpoint: `POST /admin/cache/refresh`
- Check admin permission
- Call `iamService.RefreshGroupRoleCache()`
- Return cache snapshot version

### Deliverables

- [ ] Background refresh goroutine running
- [ ] Admin API endpoint created
- [ ] Cache refresh logs visible
- [ ] Manual refresh tested

---

## Phase 8: Testing & Validation

**Priority**: P0
**Effort**: 8-10 hours
**Risk**: Low

### Objectives

- Verify race condition eliminated
- Validate all integration tests pass
- Load testing under high concurrency

### Tasks

#### Task 8.1: Run Mode 1 integration tests

```bash
make test-integration-mode1
```

Expected: 18/18 tests pass, no race conditions

#### Task 8.2: Run Mode 2 integration tests

```bash
make test-integration-mode2
```

Expected: 14/14 tests pass

#### Task 8.3: Run load tests

Scenario: 100 concurrent users, external IdP, 2 groups each

Validate:
- Request latency < 50ms
- DB write rate: 0 writes/sec during requests
- No race conditions
- No 403 errors

### Deliverables

- [ ] Mode 1 tests: 18/18 passing
- [ ] Mode 2 tests: 14/14 passing
- [ ] Load tests pass (no races, no 403s)
- [ ] Performance metrics validated

---

## Phase 9: Documentation & Cleanup

**Priority**: P2
**Effort**: 2-4 hours
**Risk**: None

### Objectives

- Updated architecture documentation
- Clean up old files
- Code review preparation

### Tasks

#### Task 9.1: Update layering.md

**File**: `cmd/gridapi/layering.md`

Add section documenting:
- IAM service layer
- Authenticator pattern
- Immutable cache pattern
- Request flow diagram

#### Task 9.2: Clean up old middleware files

Delete:
- `cmd/gridapi/internal/middleware/authn.go`
- `cmd/gridapi/internal/middleware/authn_shared.go`
- `cmd/gridapi/internal/middleware/session.go`
- `cmd/gridapi/internal/middleware/session_interceptor.go`
- `cmd/gridapi/internal/middleware/jwt_interceptor.go`
- `cmd/gridapi/internal/auth/groups.go`

### Deliverables

- [ ] layering.md updated
- [ ] Old files deleted
- [ ] Documentation reviewed
- [ ] Code review completed

---

## Success Criteria

### Core Fix (Phases 1-4):
- [x] No race conditions (go test -race passes)
- [x] Integration tests: 32/32 passing
- [x] DB query count: 2-3 per request (down from 9)
- [x] Request latency: < 50ms (down from 150ms)
- [x] Zero Casbin writes during requests

### Layering (Phases 5-6):
- [x] Zero handlers import `repository`
- [x] All services in `internal/services/`
- [x] IAM logic in `services/iam/`

### Production Ready (Phases 7-9):
- [x] Background cache refresh working
- [x] Admin API functional
- [x] Documentation updated
- [x] Code review approved

---

## Risk Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Integration tests fail | Medium | High | Run tests after each phase; rollback if failures |
| Performance regression | Low | Medium | Benchmark before/after; cache faster than mutex |
| Circular dependency | Low | Medium | Factory pattern for IAM service |
| Cache staleness | Low | Low | Background refresh + automatic on admin changes |

---

## Timeline

### Sprint 1 (Week 1): Core Architecture
- **Days 1-2**: Phase 1 (Foundation)
- **Days 3-4**: Phase 2 (Immutable Cache)
- **Day 5**: Phase 3 (Authenticators)

### Sprint 2 (Week 2): Refactoring
- **Days 6-7**: Phase 4 (Authorization)
- **Day 8**: Phase 5 (Move Services)
- **Day 9**: Phase 6 (Handlers)
- **Day 10**: Phase 8 (Testing)

### Sprint 3 (Week 3): Polish
- **Day 11**: Phase 7 (Background Refresh)
- **Day 12**: Phase 9 (Documentation)

**Total Effort**: 40-50 hours (2-3 weeks)

---

## Architecture Diagrams

### Before (Current - Race Condition)

```
Request → Session MW → JWT MW → Authn MW → ResolvePrincipal()
                                              ↓
                                        casbinMutex.Lock()
                                              ↓
                                        DB: group_roles query
                                              ↓
                                        Casbin: AddGroupingPolicy() ⚠️ WRITES
                                              ↓
                                        Casbin: GetEffectiveRoles()
                                              ↓
                                        casbinMutex.Unlock()
                                              ↓
                                        SetUserContext(principal)
                                              ↓
Handler → Authz Interceptor → Casbin.Enforce() ⚠️ RACE (state may have changed)
```

### After (Fixed - Immutable Cache)

```
Request → MultiAuth MW → Authenticator.Authenticate()
                              ↓
                         GroupRoleCache.Get() (lock-free read)
                              ↓
                         ResolveRoles() (pure function)
                              ↓
                         Principal (with Roles) → SetUserContext()
                              ↓
Handler → IAM.Authorize() → AuthorizeWithRoles(principal.Roles)
                              ↓
                         Casbin.Enforce() (read-only, no mutation)
```

---

## Key Architectural Changes

1. ✅ **Immutable cache** for group→role mappings (lock-free reads via atomic.Value)
2. ✅ **Authenticator interface** (pluggable: JWT + Session)
3. ✅ **MultiAuth middleware** (single normalization point)
4. ✅ **IAM service layer** (all auth logic centralized)
5. ✅ **Read-only Casbin** (no mutation in request path)
6. ✅ **Services directory** (proper layering: handlers → services → repositories)

---

## References

- **Race Condition Analysis**: See `specs/007-webapp-auth/GRID-80AD-FIX-SUMMARY.md`
- **Layering Rules**: See `cmd/gridapi/layering.md`
- **Integration Tests**: `tests/integration/auth_mode1_*.go`, `auth_mode2_test.go`
- **Existing Services**: `internal/state/`, `internal/dependency/`, `internal/graph/`
