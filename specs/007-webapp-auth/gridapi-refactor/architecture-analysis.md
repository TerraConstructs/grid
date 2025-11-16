# Architecture Analysis: Current State & Race Condition

**Last Updated**: 2025-11-12

## Current Authentication Flow

### Request Lifecycle (HTTP + Session Cookie)

```
1. HTTP Request
   ↓
2. Chi Router
   ↓
3. SessionMiddleware (session.go)
   - Extract "grid.session" cookie
   - DB: sessions.GetByTokenHash()
   - DB: users.GetByID()
   - Create synthetic JWT claims
   - Store in context
   ↓
4. JWTVerifier Middleware (authn.go)
   - Skip if claims exist (from session)
   - Parse Bearer token (if present)
   - Validate signature
   - Store claims in context
   ↓
5. Authn Middleware (authn.go)
   - Extract claims from context
   - Check JTI revocation
   - Call ResolvePrincipal() → RACE HAZARD ZONE
   ↓
6. ResolvePrincipal() [authn_shared.go:45-105]
   - Resolve user/service account identity
   - Extract groups from claims
   - **casbinMutex.Lock()** ⚠️
   - DB: GroupRoleRepository.GetByGroupName() for each group
   - DB: RoleRepository.GetByID() for each role
   - **Casbin: AddGroupingPolicy()** (writes to DB via AutoSave)
   - **Casbin: GetEffectiveRoles()** (reads from Casbin state)
   - **casbinMutex.Unlock()**
   - Set principal in context
   ↓
7. Handler Logic
   ↓
8. Authz Interceptor
   - Get principal from context
   - **Casbin: Enforce()** ⚠️ RACE (state may have changed)
```

## Race Condition Timeline

```
Time    Thread A (User: alice)              Thread B (User: bob)
────────────────────────────────────────────────────────────────────
T0      ResolvePrincipal()
T1        casbinMutex.Lock()
T2        clearUserGroupings(alice)
T3        AddGroupingPolicy(alice → group:X)
T4        GetEffectiveRoles(alice) = [roleX] ✓
T5        casbinMutex.Unlock()
T6                                          ResolvePrincipal()
T7                                            casbinMutex.Lock()
T8                                            clearUserGroupings(bob)
T9                                              ⚠️ ALSO clears alice's groups!
T10                                           AddGroupingPolicy(bob → group:Y)
T11                                           casbinMutex.Unlock()
T12     [Handler executes]
T13     Authz: Enforce(alice, ...)
T14       Query Casbin state
T15       ⚠️ Alice has NO groups! → 403 Forbidden
```

**Why Mutex Doesn't Help**: The mutex only protects steps 5-6 (resolution). Authorization happens later at step 8, by which time another request may have modified Casbin state.

## Database Write Amplification

### Per-Request Activity (External IdP user with 2 groups)

```sql
-- Step 1: clearUserGroupings()
SELECT v1 FROM casbin_rules WHERE ptype='g' AND v0='user:alice';
DELETE FROM casbin_rules WHERE ptype='g' AND v0='user:alice' AND v1='group:X';
DELETE FROM casbin_rules WHERE ptype='g' AND v0='user:alice' AND v1='group:Y';

-- Step 2: AddGroupingPolicy() for user→group (AutoSave triggers immediate INSERT)
INSERT INTO casbin_rules (ptype, v0, v1, ...) VALUES ('g', 'user:alice', 'group:X', ...);
INSERT INTO casbin_rules (ptype, v0, v1, ...) VALUES ('g', 'user:alice', 'group:Y', ...);

-- Step 3: AddGroupingPolicy() for group→role
INSERT INTO casbin_rules (ptype, v0, v1, ...) VALUES ('g', 'group:X', 'role:engineer', ...);
INSERT INTO casbin_rules (ptype, v0, v1, ...) VALUES ('g', 'group:Y', 'role:dev', ...);

-- Step 4: GetEffectiveRoles()
SELECT v1 FROM casbin_rules WHERE ptype='g' AND v0='user:alice';
SELECT v1 FROM casbin_rules WHERE ptype='g' AND v0='group:X';
SELECT v1 FROM casbin_rules WHERE ptype='g' AND v0='group:Y';
```

**Total**: 3 SELECTs + 2 DELETEs + 4 INSERTs = **9 queries per authenticated request**

### Load Impact (100 concurrent users @ 1 req/s)

- DELETE operations: 200/sec
- INSERT operations: 400/sec
- SELECT operations: 300/sec
- **Total**: 900 queries/sec to `casbin_rules` table
- **Index contention**: On `(ptype, v0, v1)` composite key
- **Potential deadlocks**: Under high concurrency

## Current Service Structure

```
cmd/gridapi/internal/
├── auth/                        # Auth utilities (15 files)
│   ├── casbin.go               # Enforcer initialization
│   ├── groups.go               # ⚠️ ApplyDynamicGroupings (JIT mutation)
│   ├── jwt.go                  # JWT verification
│   ├── claims.go               # Claim extraction
│   └── ...
├── middleware/                  # 7 files, 1521 LOC
│   ├── authn.go                # ⚠️ JWT auth + principal resolution
│   ├── authn_shared.go         # ⚠️ ResolvePrincipal() with casbinMutex
│   ├── session.go              # ⚠️ Session cookie auth (HTTP)
│   ├── session_interceptor.go  # ⚠️ Session cookie auth (Connect)
│   ├── jwt_interceptor.go      # JWT auth (Connect)
│   ├── authz.go                # Authorization (HTTP)
│   └── authz_interceptor.go    # Authorization (Connect)
├── server/                      # Handlers
│   ├── auth_handlers.go        # ⚠️ 6 layering violations
│   ├── auth_helpers.go         # ⚠️ 9 layering violations (duplicate logic)
│   └── ...
├── state/                       # State service (should be in services/)
├── dependency/                  # Dependency service (should be in services/)
├── graph/                       # Graph algorithms (should be in services/)
└── tfstate/                     # TF state parsing (should be in services/)
```

## Layering Violations (26 total)

### Handlers → Repositories (Should be Handlers → Services)

**File**: `auth_handlers.go`
1. Line 60: `deps.Users.GetBySubject()` - User lookup
2. Line 69: `deps.Users.Create()` - User creation (JIT provisioning)
3. Line 84: `deps.Sessions.Create()` - Session creation
4. Line 243: `deps.Users.GetByEmail()` - Email lookup
5. Line 272: `deps.Sessions.Create()` - Login session creation
6. Line 338: `deps.Sessions.Revoke()` - Logout

**File**: `auth_helpers.go`
7-15. Nine violations: Direct repository access for role resolution

### Middleware → Repositories (Should be Middleware → Services)

**File**: `authn_shared.go`
16. Line 74: `deps.GroupRoles.GetByGroupName()` - Group role lookup
17. Line 303: `deps.Users.GetByID()` - User lookup (JIT)
18. Line 313: `deps.ServiceAccounts.GetByClientID()` - SA lookup

**File**: `session.go`
19. Line 58: `deps.Sessions.GetByTokenHash()` - Session lookup
20. Line 87: `deps.Users.GetByID()` - User lookup
21. Line 103: `deps.Sessions.UpdateLastUsed()` - Session update
22. Line 148: `deps.RevokedJTIs.IsRevoked()` - Revocation check

**File**: `session_interceptor.go`
23-26. Four violations: Same pattern as session.go

## Group→Role Resolution Flow

Current implementation rebuilds group→role map **per request**:

```go
// authn_shared.go:buildGroupRoleMap()
func buildGroupRoleMap(ctx, deps, groups []string) (map[string][]string, error) {
    groupRoleMap := make(map[string][]string)
    roleNameCache := make(map[string]string)

    for _, group := range groups {
        // DB Query #1: Get group role assignments
        assignments, err := deps.GroupRoles.GetByGroupName(ctx, group)

        for _, assignment := range assignments {
            // DB Query #2: Get role name
            role, err := deps.Roles.GetByID(ctx, assignment.RoleID)
            roleNameCache[assignment.RoleID] = role.Name

            groupRoleMap[group] = append(groupRoleMap[group], role.Name)
        }
    }

    return groupRoleMap, nil
}
```

**Problem**: This is **pure read logic** that should be cached, not recomputed on every request.

## Casbin State Mutation

```go
// auth/groups.go:ApplyDynamicGroupings()
func ApplyDynamicGroupings(enforcer, userPrincipal string, groups []string, groupRoleMap) error {
    // Step 1: Clear old user groupings
    clearUserGroupings(enforcer, userPrincipal)  // DELETEs from DB

    // Step 2: Add user→group policies
    for _, group := range groups {
        enforcer.AddGroupingPolicy(userPrincipal, GroupID(group))  // INSERTs to DB
    }

    // Step 3: Add group→role policies
    for group, roles := range groupRoleMap {
        for _, role := range roles {
            enforcer.AddGroupingPolicy(GroupID(group), RoleID(role))  // INSERTs to DB
        }
    }

    return nil
}
```

**Problem**: Casbin is designed for **static policies**, not per-request temporary state. This violates Casbin's intended usage.

## Why This Causes Race Conditions

1. **Global Mutable State**: Casbin enforcer is shared across all goroutines
2. **No Transaction Isolation**: Steps are:
   - T1: Resolve principal (write to Casbin)
   - T2: Handler logic
   - T3: Authorize (read from Casbin)
   - Between T1 and T3, another request can modify Casbin!

3. **AutoSave Compounds Problem**: Every policy change immediately writes to DB, amplifying writes and holding locks longer

## Existing Workarounds (Insufficient)

### casbinMutex (Added in GRID-80AD fix)

```go
// authn_shared.go:16
var casbinMutex sync.Mutex

// Line 81-101
casbinMutex.Lock()
defer casbinMutex.Unlock()
// Apply groupings + resolve roles
```

**Why Insufficient**:
- Only protects resolution phase
- Doesn't protect authorization phase (happens later)
- Creates bottleneck under high concurrency
- Bandaid over architectural problem

## Correct Architecture (Target State)

### Immutable Cache Pattern

```
GroupRoleCache
    ├── snapshot: atomic.Value → *GroupRoleSnapshot
    │   ├── Mappings: map[string][]string (immutable)
    │   ├── CreatedAt: time.Time
    │   └── Version: int
    │
    ├── Get() → *GroupRoleSnapshot
    │   └── return snapshot.Load() // Lock-free!
    │
    └── Refresh(ctx)
        ├── newMappings := LoadFromDB()
        ├── newSnapshot := &GroupRoleSnapshot{Mappings: newMappings, ...}
        └── snapshot.Store(newSnapshot) // Atomic swap!
```

### Request Flow (Fixed)

```
Request → MultiAuth → Authenticator.Authenticate()
                        ↓
                   cache.Get() // Lock-free read
                        ↓
                   ResolveRoles() // Pure function
                        ↓
                   Principal{Roles: [...]} // Immutable
                        ↓
                   SetUserContext()
                        ↓
Handler → IAM.Authorize(principal)
                        ↓
            AuthorizeWithRoles(principal.Roles, ...)
                        ↓
            Casbin.Enforce() // Read-only, no mutation
```

**Key Improvements**:
- ✅ No Casbin mutation during requests
- ✅ No database writes during requests
- ✅ Lock-free cache reads (atomic.Value)
- ✅ Roles computed once, stored in Principal
- ✅ No race conditions possible

## Performance Analysis

### Current System (Before)

| Operation | Time | DB Queries | Locks |
|-----------|------|------------|-------|
| ResolvePrincipal | 50-100ms | 6-8 queries | Mutex held |
| Authorization | 10-20ms | 2-3 queries | Mutex contention |
| **Total per request** | **60-120ms** | **8-11 queries** | **High contention** |

### Target System (After)

| Operation | Time | DB Queries | Locks |
|-----------|------|------------|-------|
| Authenticate | 10-20ms | 2-3 queries | None |
| ResolveRoles | <1ms | 0 (cache hit) | None |
| Authorization | 5-10ms | 0-1 queries | None |
| **Total per request** | **15-30ms** | **2-4 queries** | **Zero contention** |

**Improvement**: 4x faster, 70% fewer queries, zero lock contention

## Related Documents

- **Solution Overview**: [overview.md](overview.md)
- **Implementation Phases**: [phase-*.md](.)
- **Timeline**: [timeline-and-risks.md](timeline-and-risks.md)
