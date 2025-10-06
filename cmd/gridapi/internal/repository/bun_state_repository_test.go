package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Test database connection string
const testDBURL = "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"

// setupTestDB creates a test database connection and ensures the states table exists
func setupTestDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(testDBURL)))
	db := bun.NewDB(sqldb, pgdialect.New())

	// Ensure the table exists (should be created by migrations)
	ctx := context.Background()
	_, err := db.NewSelect().Table("states").Limit(1).Exec(ctx)
	if err != nil {
		t.Skipf("Database not available or states table missing: %v", err)
	}

	return db
}

// cleanupTestData removes all test data from the states table
func cleanupTestData(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx := context.Background()
	_, err := db.NewDelete().Table("states").Where("logic_id LIKE ?", "test-%").Exec(ctx)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test data: %v", err)
	}
}

func TestBunStateRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewBunStateRepository(db)
	ctx := context.Background()

	t.Run("create valid state", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}

		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Verify it was created
		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.Equal(t, state.GUID, retrieved.GUID)
		assert.Equal(t, state.LogicID, retrieved.LogicID)
		assert.False(t, retrieved.Locked)
		assert.Nil(t, retrieved.LockInfo)
		assert.NotZero(t, retrieved.CreatedAt)
		assert.NotZero(t, retrieved.UpdatedAt)
	})

	t.Run("create with invalid UUID", func(t *testing.T) {
		state := &models.State{
			GUID:    "not-a-uuid",
			LogicID: "test-invalid",
		}

		err := repo.Create(ctx, state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "guid must be a valid UUID")
	})

	t.Run("create with empty logic_id", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "",
		}

		err := repo.Create(ctx, state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "logic_id is required")
	})

	t.Run("create with duplicate logic_id", func(t *testing.T) {
		logicID := "test-" + uuid.NewString()[:8]

		state1 := &models.State{
			GUID:    uuid.NewString(),
			LogicID: logicID,
		}
		err := repo.Create(ctx, state1)
		require.NoError(t, err)

		state2 := &models.State{
			GUID:    uuid.NewString(),
			LogicID: logicID,
		}
		err = repo.Create(ctx, state2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestBunStateRepository_GetByGUID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewBunStateRepository(db)
	ctx := context.Background()

	t.Run("get existing state", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.Equal(t, state.GUID, retrieved.GUID)
		assert.Equal(t, state.LogicID, retrieved.LogicID)
	})

	t.Run("get non-existent state", func(t *testing.T) {
		nonExistentGUID := uuid.NewString()

		_, err := repo.GetByGUID(ctx, nonExistentGUID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestBunStateRepository_GetByLogicID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewBunStateRepository(db)
	ctx := context.Background()

	t.Run("get existing state", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		retrieved, err := repo.GetByLogicID(ctx, state.LogicID)
		require.NoError(t, err)
		assert.Equal(t, state.GUID, retrieved.GUID)
		assert.Equal(t, state.LogicID, retrieved.LogicID)
	})

	t.Run("get non-existent state", func(t *testing.T) {
		_, err := repo.GetByLogicID(ctx, "non-existent-logic-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestBunStateRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewBunStateRepository(db)
	ctx := context.Background()

	t.Run("update state content", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Update content
		state.StateContent = []byte(`{"version": 4}`)
		err = repo.Update(ctx, state)
		require.NoError(t, err)

		// Verify update
		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.Equal(t, `{"version": 4}`, string(retrieved.StateContent))
	})

	t.Run("update non-existent state", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-nonexistent",
		}

		err := repo.Update(ctx, state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestBunStateRepository_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewBunStateRepository(db)
	ctx := context.Background()

	t.Run("list multiple states", func(t *testing.T) {
		// Create multiple test states
		for i := 0; i < 3; i++ {
			state := &models.State{
				GUID:    uuid.NewString(),
				LogicID: "test-list-" + uuid.NewString()[:8],
			}
			err := repo.Create(ctx, state)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		}

		states, err := repo.List(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(states), 3, "Should have at least 3 states")

		// Verify ordering (newest first)
		for i := 0; i < len(states)-1; i++ {
			assert.True(t, states[i].CreatedAt.After(states[i+1].CreatedAt) ||
				states[i].CreatedAt.Equal(states[i+1].CreatedAt),
				"States should be ordered by created_at DESC")
		}
	})

	t.Run("list empty returns empty slice", func(t *testing.T) {
		// Clean all test data
		cleanupTestData(t, db)

		states, err := repo.List(ctx)
		require.NoError(t, err)
		assert.NotNil(t, states, "Should return empty slice, not nil")
	})
}

func TestBunStateRepository_Lock_Atomicity(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewBunStateRepository(db)
	ctx := context.Background()

	t.Run("lock unlocked state", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		lockInfo := &models.LockInfo{
			ID:        uuid.NewString(),
			Operation: "apply",
			Who:       "test@localhost",
			Version:   "1.5.0",
			Created:   time.Now(),
			Path:      state.LogicID,
		}

		err = repo.Lock(ctx, state.GUID, lockInfo)
		require.NoError(t, err)

		// Verify lock was set
		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.True(t, retrieved.Locked)
		assert.NotNil(t, retrieved.LockInfo)
		assert.Equal(t, lockInfo.ID, retrieved.LockInfo.ID)
	})

	t.Run("lock already locked state fails", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// First lock
		lockInfo1 := &models.LockInfo{
			ID:        uuid.NewString(),
			Operation: "apply",
			Who:       "user1@localhost",
			Version:   "1.5.0",
			Created:   time.Now(),
			Path:      state.LogicID,
		}
		err = repo.Lock(ctx, state.GUID, lockInfo1)
		require.NoError(t, err)

		// Second lock attempt should fail
		lockInfo2 := &models.LockInfo{
			ID:        uuid.NewString(),
			Operation: "plan",
			Who:       "user2@localhost",
			Version:   "1.5.0",
			Created:   time.Now(),
			Path:      state.LogicID,
		}
		err = repo.Lock(ctx, state.GUID, lockInfo2)
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "locked")

		// Verify original lock is still in place
		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.True(t, retrieved.Locked)
		assert.Equal(t, lockInfo1.ID, retrieved.LockInfo.ID)
	})

	t.Run("lock non-existent state", func(t *testing.T) {
		lockInfo := &models.LockInfo{
			ID:        uuid.NewString(),
			Operation: "apply",
			Who:       "test@localhost",
			Version:   "1.5.0",
			Created:   time.Now(),
			Path:      "nonexistent",
		}

		err := repo.Lock(ctx, uuid.NewString(), lockInfo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestBunStateRepository_Unlock(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewBunStateRepository(db)
	ctx := context.Background()

	t.Run("unlock locked state with correct ID", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Lock it first
		lockInfo := &models.LockInfo{
			ID:        uuid.NewString(),
			Operation: "apply",
			Who:       "test@localhost",
			Version:   "1.5.0",
			Created:   time.Now(),
			Path:      state.LogicID,
		}
		err = repo.Lock(ctx, state.GUID, lockInfo)
		require.NoError(t, err)

		// Unlock with correct ID
		err = repo.Unlock(ctx, state.GUID, lockInfo.ID)
		require.NoError(t, err)

		// Verify unlock
		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.False(t, retrieved.Locked)
		assert.Nil(t, retrieved.LockInfo)
	})

	t.Run("unlock with wrong lock ID fails", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Lock it
		lockInfo := &models.LockInfo{
			ID:        uuid.NewString(),
			Operation: "apply",
			Who:       "test@localhost",
			Version:   "1.5.0",
			Created:   time.Now(),
			Path:      state.LogicID,
		}
		err = repo.Lock(ctx, state.GUID, lockInfo)
		require.NoError(t, err)

		// Try to unlock with wrong ID
		wrongID := uuid.NewString()
		err = repo.Unlock(ctx, state.GUID, wrongID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mismatch")

		// Verify lock is still in place
		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.True(t, retrieved.Locked)
		assert.Equal(t, lockInfo.ID, retrieved.LockInfo.ID)
	})

	t.Run("unlock already unlocked state fails", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Try to unlock without locking first
		err = repo.Unlock(ctx, state.GUID, uuid.NewString())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not locked")
	})
}

func TestBunStateRepository_UpdateContentAndUpsertOutputs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewBunStateRepository(db)
	outputRepo := NewBunStateOutputRepository(db)
	ctx := context.Background()

	t.Run("atomic update of state and outputs", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Update with outputs
		stateContent := []byte(`{"version": 4, "serial": 1, "outputs": {"vpc_id": {"value": "vpc-123", "sensitive": false}}}`)
		outputs := []OutputKey{
			{Key: "vpc_id", Sensitive: false},
		}
		err = repo.UpdateContentAndUpsertOutputs(ctx, state.GUID, stateContent, "", 1, outputs)
		require.NoError(t, err)

		// Verify state content updated
		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.Equal(t, stateContent, retrieved.StateContent)

		// Verify outputs inserted
		cachedOutputs, err := outputRepo.GetOutputsByState(ctx, state.GUID)
		require.NoError(t, err)
		assert.Len(t, cachedOutputs, 1)
		assert.Equal(t, "vpc_id", cachedOutputs[0].Key)
		assert.False(t, cachedOutputs[0].Sensitive)
	})

	t.Run("serial-based output invalidation", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Insert initial outputs at serial 1
		stateContent1 := []byte(`{"version": 4, "serial": 1}`)
		outputs1 := []OutputKey{
			{Key: "output_a", Sensitive: false},
			{Key: "output_b", Sensitive: false},
		}
		err = repo.UpdateContentAndUpsertOutputs(ctx, state.GUID, stateContent1, "", 1, outputs1)
		require.NoError(t, err)

		// Update with serial 2 (output_a removed, output_c added)
		stateContent2 := []byte(`{"version": 4, "serial": 2}`)
		outputs2 := []OutputKey{
			{Key: "output_b", Sensitive: false},
			{Key: "output_c", Sensitive: true},
		}
		err = repo.UpdateContentAndUpsertOutputs(ctx, state.GUID, stateContent2, "", 2, outputs2)
		require.NoError(t, err)

		// Verify only serial 2 outputs exist
		cachedOutputs, err := outputRepo.GetOutputsByState(ctx, state.GUID)
		require.NoError(t, err)
		assert.Len(t, cachedOutputs, 2)

		keys := make([]string, len(cachedOutputs))
		for i, out := range cachedOutputs {
			keys[i] = out.Key
		}
		assert.Contains(t, keys, "output_b")
		assert.Contains(t, keys, "output_c")
		assert.NotContains(t, keys, "output_a")
	})

	t.Run("locked state with correct lock ID", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Lock the state
		lockInfo := &models.LockInfo{
			ID:        "lock-123",
			Operation: "apply",
			Who:       "test@localhost",
			Version:   "1.5.0",
			Created:   time.Now(),
			Path:      state.LogicID,
		}
		err = repo.Lock(ctx, state.GUID, lockInfo)
		require.NoError(t, err)

		// Update with correct lock ID should succeed
		stateContent := []byte(`{"version": 4, "serial": 1}`)
		outputs := []OutputKey{{Key: "test", Sensitive: false}}
		err = repo.UpdateContentAndUpsertOutputs(ctx, state.GUID, stateContent, "lock-123", 1, outputs)
		require.NoError(t, err)

		// Verify update succeeded
		retrieved, err := repo.GetByGUID(ctx, state.GUID)
		require.NoError(t, err)
		assert.Equal(t, stateContent, retrieved.StateContent)
	})

	t.Run("locked state without lock ID fails", func(t *testing.T) {
		state := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-" + uuid.NewString()[:8],
		}
		err := repo.Create(ctx, state)
		require.NoError(t, err)

		// Lock the state
		lockInfo := &models.LockInfo{
			ID:        "lock-456",
			Operation: "apply",
			Who:       "test@localhost",
			Version:   "1.5.0",
			Created:   time.Now(),
			Path:      state.LogicID,
		}
		err = repo.Lock(ctx, state.GUID, lockInfo)
		require.NoError(t, err)

		// Update without lock ID should fail
		stateContent := []byte(`{"version": 4, "serial": 1}`)
		outputs := []OutputKey{{Key: "test", Sensitive: false}}
		err = repo.UpdateContentAndUpsertOutputs(ctx, state.GUID, stateContent, "", 1, outputs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "locked")
	})

	t.Run("state not found", func(t *testing.T) {
		nonExistentGUID := uuid.NewString()
		stateContent := []byte(`{"version": 4, "serial": 1}`)
		outputs := []OutputKey{{Key: "test", Sensitive: false}}
		err := repo.UpdateContentAndUpsertOutputs(ctx, nonExistentGUID, stateContent, "", 1, outputs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestStateSizeWarning(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		expected bool
	}{
		{
			name:     "small state under threshold",
			size:     1024, // 1KB
			expected: false,
		},
		{
			name:     "state at threshold",
			size:     models.StateSizeWarningThreshold,
			expected: false,
		},
		{
			name:     "state just over threshold",
			size:     models.StateSizeWarningThreshold + 1,
			expected: true,
		},
		{
			name:     "large state over threshold",
			size:     15 * 1024 * 1024, // 15MB
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{
				StateContent: make([]byte, tt.size),
			}

			result := state.SizeExceedsThreshold()
			assert.Equal(t, tt.expected, result,
				"Size %d bytes should return %v", tt.size, tt.expected)
		})
	}
}
