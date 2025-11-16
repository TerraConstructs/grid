package iam

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// GroupRoleCache provides lock-free access to group→role mappings.
//
// Uses atomic.Value for zero-contention reads. The cache stores immutable
// snapshots that are never modified after creation. Refresh operations
// build a new snapshot and atomically swap the pointer.
//
// This eliminates the race condition in the old implementation where
// concurrent requests would mutate shared Casbin state.
type GroupRoleCache struct {
	snapshot      atomic.Value // Holds *GroupRoleSnapshot
	groupRoleRepo repository.GroupRoleRepository
	roleRepo      repository.RoleRepository
}

// NewGroupRoleCache creates a new cache and performs initial load from database.
//
// Returns error if initial load fails (e.g., database unavailable).
// The cache must be successfully initialized before the server can start.
func NewGroupRoleCache(groupRoleRepo repository.GroupRoleRepository, roleRepo repository.RoleRepository) (*GroupRoleCache, error) {
	cache := &GroupRoleCache{
		groupRoleRepo: groupRoleRepo,
		roleRepo:      roleRepo,
	}

	// Perform initial load (must succeed for server to start)
	ctx := context.Background()
	if err := cache.Refresh(ctx); err != nil {
		return nil, fmt.Errorf("initial cache load: %w", err)
	}

	return cache, nil
}

// Get returns the current snapshot for lock-free reads.
//
// This method never blocks and has O(1) latency. Safe for concurrent
// access from unlimited goroutines with zero contention.
//
// Returns nil if cache has never been loaded (should never happen after
// successful NewGroupRoleCache call).
func (c *GroupRoleCache) Get() *GroupRoleSnapshot {
	val := c.snapshot.Load()
	if val == nil {
		return nil
	}
	return val.(*GroupRoleSnapshot)
}

// Refresh rebuilds the cache from database and atomically swaps the snapshot.
//
// This is an out-of-band operation (not in request path). It's safe to call
// concurrently with Get() - readers will see either the old or new snapshot
// atomically, never a partial update.
//
// Called by:
//   - Server startup (NewGroupRoleCache)
//   - Background refresh goroutine (every N minutes)
//   - Admin API (manual refresh)
//   - After AssignGroupRole/RemoveGroupRole
//
// Performance: Typically 50-100ms depending on database latency and number
// of mappings. Not a concern since this runs out-of-band.
func (c *GroupRoleCache) Refresh(ctx context.Context) error {
	// Step 1: Load all group-role assignments from database
	assignments, err := c.groupRoleRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list group roles: %w", err)
	}

	// Step 2: Build new mappings (on stack, not visible to readers yet)
	newMappings := make(map[string][]string)
	roleCache := make(map[string]string) // roleID → roleName cache

	for _, assignment := range assignments {
		// Lookup role name (with cache to minimize DB queries)
		roleName, ok := roleCache[assignment.RoleID]
		if !ok {
			role, err := c.roleRepo.GetByID(ctx, assignment.RoleID)
			if err != nil {
				return fmt.Errorf("get role %s: %w", assignment.RoleID, err)
			}
			roleName = role.Name
			roleCache[assignment.RoleID] = roleName
		}

		// Add to mappings (group can have multiple roles)
		newMappings[assignment.GroupName] = append(newMappings[assignment.GroupName], roleName)
	}

	// Step 3: Get previous version for incrementing
	prevVersion := 0
	if prev := c.snapshot.Load(); prev != nil {
		prevVersion = prev.(*GroupRoleSnapshot).Version
	}

	// Step 4: Create immutable snapshot
	newSnapshot := &GroupRoleSnapshot{
		Mappings:  newMappings,
		CreatedAt: time.Now(),
		Version:   prevVersion + 1,
	}

	// Step 5: Atomic swap - all readers see new snapshot immediately
	c.snapshot.Store(newSnapshot)

	return nil
}

// GetRolesForGroups computes the union of roles for the given groups.
//
// This is a PURE FUNCTION with no side effects:
//   - No database queries
//   - No state mutation
//   - Uses lock-free cache read (Get())
//
// Returns deduplicated list of role names. If a role is granted by multiple
// groups, it appears once in the result.
//
// Examples:
//   - GetRolesForGroups(["platform-engineers"]) → ["platform-engineer"]
//   - GetRolesForGroups(["platform-engineers", "dev-team"]) → ["platform-engineer", "product-engineer"]
//   - GetRolesForGroups([]) → []
//   - GetRolesForGroups(["unknown-group"]) → []
func (c *GroupRoleCache) GetRolesForGroups(groups []string) []string {
	snapshot := c.Get()
	if snapshot == nil {
		return []string{}
	}

	// Use map for deduplication
	roleSet := make(map[string]struct{})

	for _, groupName := range groups {
		if roles, ok := snapshot.Mappings[groupName]; ok {
			for _, role := range roles {
				roleSet[role] = struct{}{}
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(roleSet))
	for role := range roleSet {
		result = append(result, role)
	}

	return result
}
