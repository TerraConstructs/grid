# Phase 6: Handler Refactor

**Priority**: P1
**Effort**: 4-6 hours
**Risk**: Medium
**Dependencies**: Phase 5 complete

## Objectives

- Fix all 26 layering violations
- Handlers use services, not repositories
- Delete auth_helpers.go

## Tasks

### Task 6.1: Refactor Auth Handlers

**File**: `server/auth_handlers.go`

**Changes** (6 violations):

| Line | Before | After |
|------|--------|-------|
| 60 | `deps.Users.GetBySubject()` | `iamService.GetUserBySubject()` |
| 69 | `deps.Users.Create()` | `iamService.CreateUser()` |
| 84 | `deps.Sessions.Create()` | `iamService.CreateSession()` |
| 243 | `deps.Users.GetByEmail()` | `iamService.GetUserByEmail()` |
| 272 | `deps.Sessions.Create()` | `iamService.CreateSession()` |
| 338 | `deps.Sessions.Revoke()` | `iamService.RevokeSession()` |

### Task 6.2: Delete auth_helpers.go

**File**: `server/auth_helpers.go`

Move logic to IAM service:
- `resolveEffectiveRoles()` → Already in `iamService.ResolveRoles()`
- `buildGroupRoleMap()` → Already in `GroupRoleCache`

Delete the entire file.

### Task 6.3: Update Handler Dependencies

**File**: `server/deps.go` (create if doesn't exist)

**Before**:
```go
type Deps struct {
    Users           repository.UserRepository
    Sessions        repository.SessionRepository
    // ... 15+ repository dependencies
}
```

**After**:
```go
type Deps struct {
    IAMService        iam.Service
    StateService      *state.Service
    DependencyService *dependency.Service
}
```

### Task 6.4: Update All Handler Functions

Update function signatures to use services:

```go
// Before
func HandleInternalLogin(deps *Deps) http.HandlerFunc { ... }

// After
func HandleInternalLogin(iamService iam.Service) http.HandlerFunc { ... }
```

### Task 6.5: Update Router Wiring

**File**: `server/router.go`

Update handler registration to pass services instead of raw dependencies.

## Validation

After this phase:
- [ ] `grep -r "repository\." server/` returns ZERO results in handler logic
- [ ] All handlers use only services
- [ ] auth_helpers.go deleted
- [ ] All 32 integration tests pass

## Deliverables

- [ ] Handlers refactored to use IAM service
- [ ] auth_helpers.go deleted
- [ ] Handler dependencies updated
- [ ] Zero handlers import `repository` package
- [ ] All tests pass

## Related Documents

- **Previous**: [phase-5-move-services.md](phase-5-move-services.md)
- **Next**: [phase-7-cache-refresh.md](phase-7-cache-refresh.md)
