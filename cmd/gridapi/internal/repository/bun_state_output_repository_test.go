package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// cleanupTestOutputs removes all test output data
func cleanupTestOutputs(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx := context.Background()
	_, err := db.NewDelete().Table("state_outputs").Where("1=1").Exec(ctx)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test output data: %v", err)
	}
}

func TestBunStateOutputRepository_UpsertOutputs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	defer cleanupTestOutputs(t, db)

	repo := NewBunStateOutputRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	// Create a test state
	testState := &models.State{
		GUID:    uuid.NewString(),
		LogicID: "test-" + uuid.NewString()[:8],
	}
	err := stateRepo.Create(ctx, testState)
	require.NoError(t, err)

	t.Run("insert new outputs", func(t *testing.T) {
		outputs := []OutputKey{
			{Key: "vpc_id", Sensitive: false},
			{Key: "db_password", Sensitive: true},
		}

		err := repo.UpsertOutputs(ctx, testState.GUID, 1, outputs)
		require.NoError(t, err)

		// Verify they were inserted
		retrieved, err := repo.GetOutputsByState(ctx, testState.GUID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 2)

		// Check they're sorted by key
		assert.Equal(t, "db_password", retrieved[0].Key)
		assert.True(t, retrieved[0].Sensitive)
		assert.Equal(t, "vpc_id", retrieved[1].Key)
		assert.False(t, retrieved[1].Sensitive)
	})

	t.Run("upsert with new serial invalidates old", func(t *testing.T) {
		// Initial outputs at serial 1
		outputs1 := []OutputKey{
			{Key: "output_a", Sensitive: false},
			{Key: "output_b", Sensitive: false},
		}
		err := repo.UpsertOutputs(ctx, testState.GUID, 1, outputs1)
		require.NoError(t, err)

		// New outputs at serial 2 (output_a removed, output_c added)
		outputs2 := []OutputKey{
			{Key: "output_b", Sensitive: false},
			{Key: "output_c", Sensitive: true},
		}
		err = repo.UpsertOutputs(ctx, testState.GUID, 2, outputs2)
		require.NoError(t, err)

		// Should only have serial 2 outputs
		retrieved, err := repo.GetOutputsByState(ctx, testState.GUID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 2)

		keys := make([]string, len(retrieved))
		for i, out := range retrieved {
			keys[i] = out.Key
		}
		assert.Contains(t, keys, "output_b")
		assert.Contains(t, keys, "output_c")
		assert.NotContains(t, keys, "output_a") // Should be deleted
	})

	t.Run("upsert empty outputs", func(t *testing.T) {
		// Insert some outputs first
		outputs := []OutputKey{{Key: "temp", Sensitive: false}}
		err := repo.UpsertOutputs(ctx, testState.GUID, 3, outputs)
		require.NoError(t, err)

		// Upsert empty list should delete old serial but not insert anything
		err = repo.UpsertOutputs(ctx, testState.GUID, 4, []OutputKey{})
		require.NoError(t, err)

		// Should have no outputs (old serial deleted, nothing inserted)
		retrieved, err := repo.GetOutputsByState(ctx, testState.GUID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 0)
	})

	t.Run("idempotent upsert same serial", func(t *testing.T) {
		outputs := []OutputKey{
			{Key: "idempotent_test", Sensitive: false},
		}

		// Insert twice with same serial
		err := repo.UpsertOutputs(ctx, testState.GUID, 10, outputs)
		require.NoError(t, err)

		err = repo.UpsertOutputs(ctx, testState.GUID, 10, outputs)
		require.NoError(t, err)

		// Should only have one entry
		retrieved, err := repo.GetOutputsByState(ctx, testState.GUID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 1)
		assert.Equal(t, "idempotent_test", retrieved[0].Key)
	})
}

func TestBunStateOutputRepository_GetOutputsByState(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	defer cleanupTestOutputs(t, db)

	repo := NewBunStateOutputRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	// Create test state
	testState := &models.State{
		GUID:    uuid.NewString(),
		LogicID: "test-" + uuid.NewString()[:8],
	}
	err := stateRepo.Create(ctx, testState)
	require.NoError(t, err)

	t.Run("get outputs for state with outputs", func(t *testing.T) {
		outputs := []OutputKey{
			{Key: "alpha", Sensitive: false},
			{Key: "beta", Sensitive: true},
			{Key: "gamma", Sensitive: false},
		}
		err := repo.UpsertOutputs(ctx, testState.GUID, 1, outputs)
		require.NoError(t, err)

		retrieved, err := repo.GetOutputsByState(ctx, testState.GUID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 3)

		// Should be sorted by key
		assert.Equal(t, "alpha", retrieved[0].Key)
		assert.Equal(t, "beta", retrieved[1].Key)
		assert.Equal(t, "gamma", retrieved[2].Key)
	})

	t.Run("get outputs for state with no outputs", func(t *testing.T) {
		emptyState := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-empty-" + uuid.NewString()[:8],
		}
		err := stateRepo.Create(ctx, emptyState)
		require.NoError(t, err)

		retrieved, err := repo.GetOutputsByState(ctx, emptyState.GUID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 0)
		assert.NotNil(t, retrieved) // Should be empty slice, not nil
	})
}

func TestBunStateOutputRepository_SearchOutputsByKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	defer cleanupTestOutputs(t, db)

	repo := NewBunStateOutputRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	// Create multiple states with same output key
	state1 := &models.State{
		GUID:    uuid.NewString(),
		LogicID: "test-search-1-" + uuid.NewString()[:4],
	}
	state2 := &models.State{
		GUID:    uuid.NewString(),
		LogicID: "test-search-2-" + uuid.NewString()[:4],
	}
	state3 := &models.State{
		GUID:    uuid.NewString(),
		LogicID: "test-search-3-" + uuid.NewString()[:4],
	}

	err := stateRepo.Create(ctx, state1)
	require.NoError(t, err)
	err = stateRepo.Create(ctx, state2)
	require.NoError(t, err)
	err = stateRepo.Create(ctx, state3)
	require.NoError(t, err)

	t.Run("search for common output key", func(t *testing.T) {
		// State1 and State2 both have "vpc_id", State3 has different output
		err := repo.UpsertOutputs(ctx, state1.GUID, 1, []OutputKey{{Key: "vpc_id", Sensitive: false}})
		require.NoError(t, err)

		err = repo.UpsertOutputs(ctx, state2.GUID, 1, []OutputKey{
			{Key: "vpc_id", Sensitive: true},  // Same key, different sensitive flag
			{Key: "subnet_id", Sensitive: false},
		})
		require.NoError(t, err)

		err = repo.UpsertOutputs(ctx, state3.GUID, 1, []OutputKey{{Key: "other_output", Sensitive: false}})
		require.NoError(t, err)

		// Search for "vpc_id"
		results, err := repo.SearchOutputsByKey(ctx, "vpc_id")
		require.NoError(t, err)
		assert.Len(t, results, 2)

		// Verify we got state1 and state2
		guids := []string{results[0].StateGUID, results[1].StateGUID}
		assert.Contains(t, guids, state1.GUID)
		assert.Contains(t, guids, state2.GUID)
		assert.NotContains(t, guids, state3.GUID)

		// Verify logic IDs are populated
		for _, result := range results {
			assert.NotEmpty(t, result.StateLogicID)
			assert.Equal(t, "vpc_id", result.OutputKey)
		}
	})

	t.Run("search for non-existent output key", func(t *testing.T) {
		results, err := repo.SearchOutputsByKey(ctx, "nonexistent_key")
		require.NoError(t, err)
		assert.Len(t, results, 0)
		assert.NotNil(t, results) // Should be empty slice, not nil
	})
}

func TestBunStateOutputRepository_DeleteOutputsByState(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	defer cleanupTestOutputs(t, db)

	repo := NewBunStateOutputRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	testState := &models.State{
		GUID:    uuid.NewString(),
		LogicID: "test-delete-" + uuid.NewString()[:8],
	}
	err := stateRepo.Create(ctx, testState)
	require.NoError(t, err)

	t.Run("delete outputs for state with outputs", func(t *testing.T) {
		outputs := []OutputKey{
			{Key: "to_delete_1", Sensitive: false},
			{Key: "to_delete_2", Sensitive: true},
		}
		err := repo.UpsertOutputs(ctx, testState.GUID, 1, outputs)
		require.NoError(t, err)

		// Verify they exist
		retrieved, err := repo.GetOutputsByState(ctx, testState.GUID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 2)

		// Delete them
		err = repo.DeleteOutputsByState(ctx, testState.GUID)
		require.NoError(t, err)

		// Verify they're gone
		retrieved, err = repo.GetOutputsByState(ctx, testState.GUID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 0)
	})

	t.Run("delete outputs for state with no outputs", func(t *testing.T) {
		emptyState := &models.State{
			GUID:    uuid.NewString(),
			LogicID: "test-delete-empty-" + uuid.NewString()[:8],
		}
		err := stateRepo.Create(ctx, emptyState)
		require.NoError(t, err)

		// Should not error on deleting non-existent outputs
		err = repo.DeleteOutputsByState(ctx, emptyState.GUID)
		require.NoError(t, err)
	})
}

func TestBunStateOutputRepository_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	defer cleanupTestOutputs(t, db)

	repo := NewBunStateOutputRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	testState := &models.State{
		GUID:    uuid.NewString(),
		LogicID: "test-cascade-" + uuid.NewString()[:8],
	}
	err := stateRepo.Create(ctx, testState)
	require.NoError(t, err)

	// Add outputs
	outputs := []OutputKey{
		{Key: "cascade_test", Sensitive: false},
	}
	err = repo.UpsertOutputs(ctx, testState.GUID, 1, outputs)
	require.NoError(t, err)

	// Verify outputs exist
	retrieved, err := repo.GetOutputsByState(ctx, testState.GUID)
	require.NoError(t, err)
	assert.Len(t, retrieved, 1)

	// Delete the state (CASCADE should delete outputs)
	_, err = db.NewDelete().Model((*models.State)(nil)).Where("guid = ?", testState.GUID).Exec(ctx)
	require.NoError(t, err)

	// Verify outputs were cascade deleted
	retrieved, err = repo.GetOutputsByState(ctx, testState.GUID)
	require.NoError(t, err)
	assert.Len(t, retrieved, 0)
}
