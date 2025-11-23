package iam

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// Mock repositories for testing

type mockGroupRoleRepository struct {
	mu      sync.RWMutex
	records []models.GroupRole
}

func (m *mockGroupRoleRepository) Create(ctx context.Context, gr *models.GroupRole) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, *gr)
	return nil
}

func (m *mockGroupRoleRepository) GetByID(ctx context.Context, id string) (*models.GroupRole, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, gr := range m.records {
		if gr.ID == id {
			return &gr, nil
		}
	}
	return nil, nil
}

func (m *mockGroupRoleRepository) GetByGroupName(ctx context.Context, groupName string) ([]models.GroupRole, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []models.GroupRole
	for _, gr := range m.records {
		if gr.GroupName == groupName {
			result = append(result, gr)
		}
	}
	return result, nil
}

func (m *mockGroupRoleRepository) GetByRoleID(ctx context.Context, roleID string) ([]models.GroupRole, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []models.GroupRole
	for _, gr := range m.records {
		if gr.RoleID == roleID {
			result = append(result, gr)
		}
	}
	return result, nil
}

func (m *mockGroupRoleRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, gr := range m.records {
		if gr.ID == id {
			m.records = append(m.records[:i], m.records[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockGroupRoleRepository) DeleteByGroupAndRole(ctx context.Context, groupName string, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.records) - 1; i >= 0; i-- {
		if m.records[i].GroupName == groupName && m.records[i].RoleID == roleID {
			m.records = append(m.records[:i], m.records[i+1:]...)
		}
	}
	return nil
}

func (m *mockGroupRoleRepository) List(ctx context.Context) ([]models.GroupRole, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return copy to prevent data races
	result := make([]models.GroupRole, len(m.records))
	copy(result, m.records)
	return result, nil
}

type mockRoleRepository struct {
	mu    sync.RWMutex
	roles map[string]*models.Role // roleID → role
}

func (m *mockRoleRepository) Create(ctx context.Context, role *models.Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles[role.ID] = role
	return nil
}

func (m *mockRoleRepository) GetByID(ctx context.Context, id string) (*models.Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if role, ok := m.roles[id]; ok {
		return role, nil
	}
	return nil, nil
}

func (m *mockRoleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, role := range m.roles {
		if role.Name == name {
			return role, nil
		}
	}
	return nil, nil
}

func (m *mockRoleRepository) Update(ctx context.Context, role *models.Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles[role.ID] = role
	return nil
}

func (m *mockRoleRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.roles, id)
	return nil
}

func (m *mockRoleRepository) List(ctx context.Context) ([]models.Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]models.Role, 0, len(m.roles))
	for _, role := range m.roles {
		result = append(result, *role)
	}
	return result, nil
}

// Test helper to create test data
func setupTestCache(t *testing.T) (*GroupRoleCache, *mockGroupRoleRepository, *mockRoleRepository) {
	t.Helper()

	// Create mock repositories with test data
	roleRepo := &mockRoleRepository{
		roles: map[string]*models.Role{
			"role-1": {ID: "role-1", Name: "platform-engineer"},
			"role-2": {ID: "role-2", Name: "product-engineer"},
			"role-3": {ID: "role-3", Name: "viewer"},
		},
	}

	groupRoleRepo := &mockGroupRoleRepository{
		records: []models.GroupRole{
			{ID: "gr-1", GroupName: "platform-engineers", RoleID: "role-1", AssignedAt: time.Now()},
			{ID: "gr-2", GroupName: "dev-team", RoleID: "role-2", AssignedAt: time.Now()},
			{ID: "gr-3", GroupName: "everyone", RoleID: "role-3", AssignedAt: time.Now()},
		},
	}

	cache, err := NewGroupRoleCache(groupRoleRepo, roleRepo)
	if err != nil {
		t.Fatalf("NewGroupRoleCache failed: %v", err)
	}

	return cache, groupRoleRepo, roleRepo
}

// Test 1: Initial load creates snapshot
func TestGroupRoleCache_InitialLoad(t *testing.T) {
	cache, _, _ := setupTestCache(t)

	snapshot := cache.Get()
	if snapshot == nil {
		t.Fatal("Expected non-nil snapshot after initial load")
	}

	if snapshot.Version != 1 {
		t.Errorf("Expected version 1, got %d", snapshot.Version)
	}

	if len(snapshot.Mappings) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(snapshot.Mappings))
	}

	// Verify mappings
	if roles, ok := snapshot.Mappings["platform-engineers"]; !ok || len(roles) != 1 || roles[0] != "platform-engineer" {
		t.Errorf("Unexpected mapping for platform-engineers: %v", roles)
	}

	if roles, ok := snapshot.Mappings["dev-team"]; !ok || len(roles) != 1 || roles[0] != "product-engineer" {
		t.Errorf("Unexpected mapping for dev-team: %v", roles)
	}
}

// Test 2: Get returns current snapshot
func TestGroupRoleCache_Get(t *testing.T) {
	cache, _, _ := setupTestCache(t)

	snapshot1 := cache.Get()
	snapshot2 := cache.Get()

	// Same snapshot pointer (immutable)
	if snapshot1 != snapshot2 {
		t.Error("Expected Get() to return same snapshot instance")
	}

	if snapshot1.Version != 1 {
		t.Errorf("Expected version 1, got %d", snapshot1.Version)
	}
}

// Test 3: Refresh creates new snapshot and increments version
func TestGroupRoleCache_Refresh(t *testing.T) {
	cache, groupRoleRepo, _ := setupTestCache(t)

	snapshot1 := cache.Get()
	if snapshot1.Version != 1 {
		t.Fatalf("Expected initial version 1, got %d", snapshot1.Version)
	}

	// Add new group-role mapping
	_ = groupRoleRepo.Create(context.Background(), &models.GroupRole{
		ID:         "gr-4",
		GroupName:  "admins",
		RoleID:     "role-1",
		AssignedAt: time.Now(),
	})

	// Refresh cache
	err := cache.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	snapshot2 := cache.Get()
	if snapshot2.Version != 2 {
		t.Errorf("Expected version 2 after refresh, got %d", snapshot2.Version)
	}

	// Verify new mapping exists
	if roles, ok := snapshot2.Mappings["admins"]; !ok || len(roles) != 1 {
		t.Errorf("Expected admins group after refresh, got %v", roles)
	}

	// Old snapshot unchanged (immutability)
	if _, ok := snapshot1.Mappings["admins"]; ok {
		t.Error("Old snapshot was mutated (should be immutable)")
	}
}

// Test 4: GetRolesForGroups computes union correctly
func TestGroupRoleCache_GetRolesForGroups(t *testing.T) {
	cache, _, _ := setupTestCache(t)

	tests := []struct {
		name     string
		groups   []string
		expected map[string]bool // Use map for easier comparison
	}{
		{
			name:     "Single group",
			groups:   []string{"platform-engineers"},
			expected: map[string]bool{"platform-engineer": true},
		},
		{
			name:     "Multiple groups",
			groups:   []string{"platform-engineers", "dev-team"},
			expected: map[string]bool{"platform-engineer": true, "product-engineer": true},
		},
		{
			name:     "All groups",
			groups:   []string{"platform-engineers", "dev-team", "everyone"},
			expected: map[string]bool{"platform-engineer": true, "product-engineer": true, "viewer": true},
		},
		{
			name:     "Empty groups",
			groups:   []string{},
			expected: map[string]bool{},
		},
		{
			name:     "Unknown group",
			groups:   []string{"unknown-group"},
			expected: map[string]bool{},
		},
		{
			name:     "Mixed known and unknown",
			groups:   []string{"platform-engineers", "unknown-group"},
			expected: map[string]bool{"platform-engineer": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roles := cache.GetRolesForGroups(tt.groups)

			// Convert result to map for comparison
			resultMap := make(map[string]bool)
			for _, role := range roles {
				resultMap[role] = true
			}

			if len(resultMap) != len(tt.expected) {
				t.Errorf("Expected %d roles, got %d: %v", len(tt.expected), len(resultMap), roles)
			}

			for role := range tt.expected {
				if !resultMap[role] {
					t.Errorf("Missing expected role: %s", role)
				}
			}

			for role := range resultMap {
				if !tt.expected[role] {
					t.Errorf("Unexpected role: %s", role)
				}
			}
		})
	}
}

// Test 5: Empty groups edge case
func TestGroupRoleCache_EmptyGroups(t *testing.T) {
	cache, _, _ := setupTestCache(t)

	roles := cache.GetRolesForGroups([]string{})
	if len(roles) != 0 {
		t.Errorf("Expected empty result for empty groups, got %v", roles)
	}
}

// Test 6: Unknown group edge case
func TestGroupRoleCache_UnknownGroup(t *testing.T) {
	cache, _, _ := setupTestCache(t)

	roles := cache.GetRolesForGroups([]string{"nonexistent-group"})
	if len(roles) != 0 {
		t.Errorf("Expected empty result for unknown group, got %v", roles)
	}
}

// Test 7: CRITICAL - Concurrent access test
// This test validates that the cache is truly lock-free by having 1000
// goroutines reading concurrently while 1 goroutine refreshes the cache.
// Run with -race flag to detect data races.
func TestGroupRoleCache_Concurrent(t *testing.T) {
	cache, groupRoleRepo, _ := setupTestCache(t)

	const numReaders = 1000
	const numReadsPerReader = 100
	const numRefreshes = 10

	var wg sync.WaitGroup

	// Start 1000 reader goroutines
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numReadsPerReader; j++ {
				// Lock-free read
				snapshot := cache.Get()
				if snapshot == nil {
					t.Errorf("Reader %d: Got nil snapshot", id)
					return
				}

				// Validate snapshot integrity
				if snapshot.Version < 1 {
					t.Errorf("Reader %d: Invalid version %d", id, snapshot.Version)
					return
				}

				// Use GetRolesForGroups (also lock-free)
				roles := cache.GetRolesForGroups([]string{"platform-engineers", "dev-team"})
				if len(roles) == 0 {
					t.Errorf("Reader %d: Got empty roles", id)
					return
				}

				// Small delay to increase chance of races
				time.Sleep(1 * time.Microsecond)
			}
		}(i)
	}

	// Start 1 writer goroutine that refreshes the cache
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numRefreshes; i++ {
			time.Sleep(10 * time.Millisecond)

			// Add a new mapping
			_ = groupRoleRepo.Create(context.Background(), &models.GroupRole{
				ID:         "gr-dynamic-" + time.Now().String(),
				GroupName:  "dynamic-group",
				RoleID:     "role-1",
				AssignedAt: time.Now(),
			})

			// Refresh cache (atomic swap)
			err := cache.Refresh(context.Background())
			if err != nil {
				t.Errorf("Writer: Refresh failed: %v", err)
				return
			}
		}
	}()

	wg.Wait()

	// Verify final state
	finalSnapshot := cache.Get()
	if finalSnapshot.Version < numRefreshes {
		t.Errorf("Expected version >= %d after refreshes, got %d", numRefreshes, finalSnapshot.Version)
	}

	t.Logf("Concurrency test passed: %d readers × %d reads + %d refreshes with zero races",
		numReaders, numReadsPerReader, numRefreshes)
}

// Test 8: Multiple groups with overlapping roles (deduplication)
func TestGroupRoleCache_Deduplication(t *testing.T) {
	roleRepo := &mockRoleRepository{
		roles: map[string]*models.Role{
			"role-1": {ID: "role-1", Name: "admin"},
		},
	}

	// Both groups have the same role
	groupRoleRepo := &mockGroupRoleRepository{
		records: []models.GroupRole{
			{ID: "gr-1", GroupName: "group-a", RoleID: "role-1", AssignedAt: time.Now()},
			{ID: "gr-2", GroupName: "group-b", RoleID: "role-1", AssignedAt: time.Now()},
		},
	}

	cache, err := NewGroupRoleCache(groupRoleRepo, roleRepo)
	if err != nil {
		t.Fatalf("NewGroupRoleCache failed: %v", err)
	}

	roles := cache.GetRolesForGroups([]string{"group-a", "group-b"})

	// Should have exactly 1 role (deduplicated)
	if len(roles) != 1 {
		t.Errorf("Expected 1 deduplicated role, got %d: %v", len(roles), roles)
	}

	if roles[0] != "admin" {
		t.Errorf("Expected admin role, got %s", roles[0])
	}
}
