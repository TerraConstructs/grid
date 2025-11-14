# Phase 4: Authorization Refactor

**Priority**: P0
**Effort**: 4-6 hours
**Risk**: Low
**Dependencies**: Phase 3 complete

## Objectives

- Remove ALL Casbin state mutation
- Use Principal.Roles as source of truth
- Implement read-only Casbin authorization
- Delete casbinMutex

## Tasks

### Task 4.1: Implement Read-Only Casbin Authorization

**File**: `services/iam/casbin_readonly.go`

```go
// AuthorizeWithRoles checks if ANY of principal's roles grants permission.
func AuthorizeWithRoles(enforcer casbin.IEnforcer, roles []string, obj, act string, labels map[string]interface{}) (bool, error) {
    for _, roleName := range roles {
        rolePrincipal := auth.RoleID(roleName) // "role:product-engineer"

        // Query Casbin enforcer (read-only, no AddGroupingPolicy!)
        allowed, err := enforcer.Enforce(rolePrincipal, obj, act, labels)
        if err != nil {
            return false, err
        }

        if allowed {
            return true, nil // At least one role allows
        }
    }

    return false, nil // No role granted permission
}
```

**IAM Service Implementation**:
```go
func (s *iamService) Authorize(ctx, principal, obj, act string, labels) (bool, error) {
    return AuthorizeWithRoles(s.enforcer, principal.Roles, obj, act, labels)
}
```

### Task 4.2: Refactor Authorization Middleware

**Files**:
- `middleware/authz_http.go` (HTTP authorization)
- `middleware/authz_connect.go` (Connect RPC authorization)

**Changes**:
1. Get Principal from context (set by MultiAuth)
2. Use `iamService.Authorize(principal, obj, act, labels)`
3. Remove ALL `ApplyDynamicGroupings()` calls
4. Remove ALL `LoadPolicy()` calls
5. Remove ALL Casbin mutation code

### Task 4.3: Remove Casbin Mutation Code

**Delete/Modify**:
1. Remove `casbinMutex` from `authn_shared.go`
2. Delete `auth/groups.go::ApplyDynamicGroupings()`
3. Delete `auth/groups.go::clearUserGroupings()`
4. Update `cmd/serve.go`: Disable AutoSave (`enforcer.EnableAutoSave(false)`)
5. Delete old middleware files:
   - `middleware/authn.go`
   - `middleware/authn_shared.go`
   - `middleware/session.go`
   - `middleware/session_interceptor.go`
   - `middleware/jwt_interceptor.go`

## Deliverables

- [ ] Read-only authorization implemented
- [ ] Authorization middleware refactored
- [ ] All Casbin mutation code removed
- [ ] casbinMutex deleted
- [ ] AutoSave disabled
- [ ] Old middleware files deleted
- [ ] All tests pass

## Validation

After this phase:
- [ ] `grep -r "AddGroupingPolicy" cmd/gridapi/` returns ZERO results
- [ ] `grep -r "casbinMutex" cmd/gridapi/` returns ZERO results
- [ ] No DB writes to `casbin_rules` during request handling

## Related Documents

- **Previous**: [phase-3-authenticator-pattern.md](phase-3-authenticator-pattern.md)
- **Next**: [phase-5-move-services.md](phase-5-move-services.md)
