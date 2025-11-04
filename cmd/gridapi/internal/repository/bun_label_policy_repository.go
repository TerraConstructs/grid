package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// BunLabelPolicyRepository manages label policy using Bun ORM.
type BunLabelPolicyRepository struct {
	db *bun.DB
}

// NewBunLabelPolicyRepository constructs a repository backed by Bun.
func NewBunLabelPolicyRepository(db *bun.DB) LabelPolicyRepository {
	return &BunLabelPolicyRepository{db: db}
}

// GetPolicy retrieves the current label policy (single-row table).
func (r *BunLabelPolicyRepository) GetPolicy(ctx context.Context) (*models.LabelPolicy, error) {
	policy := new(models.LabelPolicy)
	err := r.db.NewSelect().
		Model(policy).
		Where("id = ?", 1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("label policy not found")
		}
		return nil, fmt.Errorf("query label policy: %w", err)
	}

	return policy, nil
}

// SetPolicy creates or updates the label policy with version increment.
func (r *BunLabelPolicyRepository) SetPolicy(ctx context.Context, policyDef *models.PolicyDefinition) error {
	// Try to get existing policy
	existing, err := r.GetPolicy(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) && err.Error() != "label policy not found" {
		return fmt.Errorf("get existing policy: %w", err)
	}

	now := time.Now()

	if existing == nil {
		// Create new policy with version 1
		policy := &models.LabelPolicy{
			ID:         1,
			Version:    1,
			PolicyJSON: *policyDef,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		_, err = r.db.NewInsert().
			Model(policy).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("insert label policy: %w", err)
		}

		return nil
	}

	// Update existing policy with incremented version
	result, err := r.db.NewUpdate().
		Model((*models.LabelPolicy)(nil)).
		Set("version = version + 1").
		Set("policy_json = ?", policyDef).
		Set("updated_at = ?", now).
		Where("id = ?", 1).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update label policy: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("label policy not found")
	}

	return nil
}
