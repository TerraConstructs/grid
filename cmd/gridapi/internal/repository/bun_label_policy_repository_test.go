package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// T012: Test BunLabelPolicyRepository.GetPolicy
func TestBunLabelPolicyRepository_GetPolicy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewBunLabelPolicyRepository(db)
	ctx := context.Background()

	t.Run("get policy when none exists", func(t *testing.T) {
		// Clean any existing policy
		_, err := db.NewDelete().Table("label_policy").Where("1 = 1").Exec(ctx)
		require.NoError(t, err)

		policy, err := repo.GetPolicy(ctx)
		// Should return empty/default policy or not found error
		if err != nil {
			assert.Contains(t, err.Error(), "not found")
		} else {
			assert.NotNil(t, policy)
		}
	})

	t.Run("get existing policy", func(t *testing.T) {
		// Clean any existing policy
		_, err := db.NewDelete().Table("label_policy").Where("1 = 1").Exec(ctx)
		require.NoError(t, err)

		// Insert a policy
		policyDef := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{
				"env":  {},
				"team": {},
			},
			AllowedValues: map[string][]string{
				"env": {"staging", "prod"},
			},
			ReservedPrefixes: []string{"grid.io/"},
			MaxKeys:          32,
			MaxValueLen:      256,
		}

		err = repo.SetPolicy(ctx, policyDef)
		require.NoError(t, err)

		// Retrieve policy
		retrieved, err := repo.GetPolicy(ctx)
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, 1, retrieved.Version)
		assert.Contains(t, retrieved.PolicyJSON.AllowedKeys, "env")
		assert.Contains(t, retrieved.PolicyJSON.AllowedKeys, "team")
		assert.Equal(t, []string{"staging", "prod"}, retrieved.PolicyJSON.AllowedValues["env"])
	})
}

// T013: Test BunLabelPolicyRepository.SetPolicy
func TestBunLabelPolicyRepository_SetPolicy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewBunLabelPolicyRepository(db)
	ctx := context.Background()

	t.Run("set policy for first time", func(t *testing.T) {
		// Clean any existing policy
		_, err := db.NewDelete().Table("label_policy").Where("1 = 1").Exec(ctx)
		require.NoError(t, err)

		policyDef := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{
				"env": {},
			},
			MaxKeys:     32,
			MaxValueLen: 256,
		}

		err = repo.SetPolicy(ctx, policyDef)
		require.NoError(t, err)

		// Verify policy was created with version 1
		retrieved, err := repo.GetPolicy(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, retrieved.Version)
	})

	t.Run("update policy increments version", func(t *testing.T) {
		// Clean and set initial policy
		_, err := db.NewDelete().Table("label_policy").Where("1 = 1").Exec(ctx)
		require.NoError(t, err)

		initialPolicy := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{"env": {}},
			MaxKeys:     32,
			MaxValueLen: 256,
		}
		err = repo.SetPolicy(ctx, initialPolicy)
		require.NoError(t, err)

		// Update policy
		updatedPolicy := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{
				"env":  {},
				"team": {},
			},
			MaxKeys:     32,
			MaxValueLen: 256,
		}
		err = repo.SetPolicy(ctx, updatedPolicy)
		require.NoError(t, err)

		// Verify version incremented
		retrieved, err := repo.GetPolicy(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, retrieved.Version)
		assert.Contains(t, retrieved.PolicyJSON.AllowedKeys, "team")
	})

	t.Run("policy_json field updates correctly", func(t *testing.T) {
		// Clean
		_, err := db.NewDelete().Table("label_policy").Where("1 = 1").Exec(ctx)
		require.NoError(t, err)

		policyDef := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{
				"env":    {},
				"region": {},
			},
			AllowedValues: map[string][]string{
				"env":    {"staging", "prod", "dev"},
				"region": {"us-west", "us-east"},
			},
			ReservedPrefixes: []string{"grid.io/", "kubernetes.io/"},
			MaxKeys:          64,
			MaxValueLen:      512,
		}

		err = repo.SetPolicy(ctx, policyDef)
		require.NoError(t, err)

		// Verify all fields persisted
		retrieved, err := repo.GetPolicy(ctx)
		require.NoError(t, err)
		assert.Equal(t, 64, retrieved.PolicyJSON.MaxKeys)
		assert.Equal(t, 512, retrieved.PolicyJSON.MaxValueLen)
		assert.Len(t, retrieved.PolicyJSON.ReservedPrefixes, 2)
		assert.Equal(t, []string{"staging", "prod", "dev"}, retrieved.PolicyJSON.AllowedValues["env"])
	})
}
