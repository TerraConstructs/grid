# Phase 2: Immutable Group→Role Cache

**Priority**: P0 (Blocks Phase 3)
**Effort**: 4-6 hours
**Risk**: Low
**Dependencies**: Phase 1 complete

## Objectives

Eliminate race condition by implementing immutable cache with lock-free reads using `atomic.Value`.

## Key Design

```go
type GroupRoleCache struct {
    snapshot atomic.Value // Holds *GroupRoleSnapshot
    groupRoleRepo repository.GroupRoleRepository
    roleRepo      repository.RoleRepository
}

// Lock-free read
func (c *GroupRoleCache) Get() *GroupRoleSnapshot {
    return c.snapshot.Load().(*GroupRoleSnapshot)
}

// Atomic swap (out-of-band, not in request path)
func (c *GroupRoleCache) Refresh(ctx) error {
    newMappings := loadFromDB(ctx)
    newSnapshot := &GroupRoleSnapshot{Mappings: newMappings, Version: prev+1, ...}
    c.snapshot.Store(newSnapshot) // Atomic!
    return nil
}
```

## Tasks

### Task 2.1: Implement GroupRoleCache

**File**: `cmd/gridapi/internal/services/iam/group_role_cache.go`

**Methods**:
- `NewGroupRoleCache(groupRoleRepo, roleRepo) (*GroupRoleCache, error)` - Initialize with initial load
- `Get() *GroupRoleSnapshot` - Lock-free read via `atomic.Value.Load()`
- `Refresh(ctx context.Context) error` - Build new map, atomic swap
- `GetRolesForGroups(groups []string) []string` - Pure function, no side effects

**Key Properties**:
- ✅ Lock-free reads (zero contention)
- ✅ Immutable snapshots (never modified after creation)
- ✅ Copy-on-write refresh (builds new map, swaps pointer)
- ✅ No "clear then refill" pattern

**Pseudocode**:
```go
func (c *GroupRoleCache) Refresh(ctx context.Context) error {
    // Step 1: Load from DB (on the stack, not visible to readers)
    assignments, _ := c.groupRoleRepo.List(ctx)

    newMappings := make(map[string][]string)
    roleCache := make(map[string]string)

    for _, assignment := range assignments {
        roleName := lookupRole(assignment.RoleID, roleCache)
        newMappings[assignment.GroupName] = append(newMappings[assignment.GroupName], roleName)
    }

    // Step 2: Get previous version
    prevVersion := 0
    if prev := c.snapshot.Load(); prev != nil {
        prevVersion = prev.(*GroupRoleSnapshot).Version
    }

    // Step 3: Create new snapshot (immutable)
    newSnapshot := &GroupRoleSnapshot{
        Mappings:  newMappings,
        CreatedAt: time.Now(),
        Version:   prevVersion + 1,
    }

    // Step 4: Atomic swap (all readers see new snapshot immediately)
    c.snapshot.Store(newSnapshot)
    return nil
}
```

**Acceptance Criteria**:
- [ ] Implements all 4 methods
- [ ] Uses `atomic.Value` correctly
- [ ] Immutable snapshots (no mutation after creation)
- [ ] Compiles without errors

### Task 2.2: Write GroupRoleCache Unit Tests

**File**: `cmd/gridapi/internal/services/iam/group_role_cache_test.go`

**Test Cases**:
1. `TestGroupRoleCache_InitialLoad` - Verify initial snapshot created
2. `TestGroupRoleCache_Get` - Verify Get() returns snapshot
3. `TestGroupRoleCache_Refresh` - Verify atomic swap works
4. `TestGroupRoleCache_GetRolesForGroups` - Verify role computation correct
5. `TestGroupRoleCache_EmptyGroups` - Edge case: empty group list
6. `TestGroupRoleCache_UnknownGroup` - Edge case: group not in cache
7. `TestGroupRoleCache_Concurrent` - **Critical**: 1000 goroutines reading, 1 goroutine refreshing

**Concurrency Test**:
```go
func TestGroupRoleCache_Concurrent(t *testing.T) {
    cache := setupCache(t)

    // Start 1000 readers
    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 100; j++ {
                _ = cache.Get() // Lock-free read
                _ = cache.GetRolesForGroups([]string{"test-group"})
            }
        }()
    }

    // Start 1 writer
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 0; i < 10; i++ {
            time.Sleep(10 * time.Millisecond)
            _ = cache.Refresh(context.Background())
        }
    }()

    wg.Wait()
    // If race detector enabled, this will catch issues
}
```

**Run with race detector**: `go test -race ./...`

**Acceptance Criteria**:
- [ ] All tests pass
- [ ] Race detector passes (`go test -race`)
- [ ] 90%+ code coverage
- [ ] Concurrency test validates lock-free reads

### Task 2.3: Integrate Cache into IAM Service Implementation

**File**: `cmd/gridapi/internal/services/iam/service.go` (implementation stub)

```go
package iam

type iamService struct {
    // Repositories
    users           repository.UserRepository
    serviceAccounts repository.ServiceAccountRepository
    sessions        repository.SessionRepository
    userRoles       repository.UserRoleRepository
    groupRoles      repository.GroupRoleRepository
    roles           repository.RoleRepository
    revokedJTIs     repository.RevokedJTIRepository

    // Immutable cache (lock-free reads)
    groupRoleCache *GroupRoleCache

    // Casbin enforcer (read-only for authorization)
    enforcer casbin.IEnforcer

    // Authenticators (injected, populated in Phase 3)
    authenticators []Authenticator
}

func NewIAMService(deps IAMServiceDependencies) (Service, error) {
    // Initialize cache with initial load
    cache, err := NewGroupRoleCache(deps.GroupRoles, deps.Roles)
    if err != nil {
        return nil, fmt.Errorf("initialize group role cache: %w", err)
    }

    return &iamService{
        users:           deps.Users,
        serviceAccounts: deps.ServiceAccounts,
        sessions:        deps.Sessions,
        userRoles:       deps.UserRoles,
        groupRoles:      deps.GroupRoles,
        roles:           deps.Roles,
        revokedJTIs:     deps.RevokedJTIs,
        groupRoleCache:  cache,
        enforcer:        deps.Enforcer,
        authenticators:  []Authenticator{}, // Populated in Phase 3
    }, nil
}

// Implement ResolveRoles using cache
func (s *iamService) ResolveRoles(ctx context.Context, userID string, groups []string) ([]string, error) {
    // Step 1: Get user's directly-assigned roles (DB read)
    userRoleAssignments, err := s.userRoles.GetByUserID(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("get user roles: %w", err)
    }

    roleSet := make(map[string]struct{})

    // Add user roles
    for _, assignment := range userRoleAssignments {
        role, err := s.roles.GetByID(ctx, assignment.RoleID)
        if err != nil {
            return nil, fmt.Errorf("get role %s: %w", assignment.RoleID, err)
        }
        roleSet[role.Name] = struct{}{}
    }

    // Step 2: Get roles from groups (LOCK-FREE cache read)
    groupRoles := s.groupRoleCache.GetRolesForGroups(groups)
    for _, role := range groupRoles {
        roleSet[role] = struct{}{}
    }

    // Step 3: Convert to slice
    result := make([]string, 0, len(roleSet))
    for role := range roleSet {
        result = append(result, role)
    }

    return result, nil
}

// Implement RefreshGroupRoleCache
func (s *iamService) RefreshGroupRoleCache(ctx context.Context) error {
    return s.groupRoleCache.Refresh(ctx)
}

// Implement GetGroupRoleCacheSnapshot
func (s *iamService) GetGroupRoleCacheSnapshot() GroupRoleSnapshot {
    snapshot := s.groupRoleCache.Get()
    return *snapshot
}
```

**Acceptance Criteria**:
- [ ] IAM service struct defined
- [ ] `NewIAMService()` constructor implemented
- [ ] `ResolveRoles()` uses cache (lock-free)
- [ ] Cache refresh methods delegated
- [ ] Compiles successfully

## Performance Validation

After this phase, validate:
- [ ] Lock-free reads confirmed (no mutex contention)
- [ ] Cache hit rate > 99% (groups rarely change)
- [ ] ResolveRoles latency < 1ms (down from 50-100ms)

## Related Documents

- **Previous**: [phase-1-services-foundation.md](phase-1-services-foundation.md)
- **Next**: [phase-3-authenticator-pattern.md](phase-3-authenticator-pattern.md)
