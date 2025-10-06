package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// BunStateRepository persists states using Bun ORM against PostgreSQL.
type BunStateRepository struct {
	db *bun.DB
}

// NewBunStateRepository constructs a repository backed by Bun.
func NewBunStateRepository(db *bun.DB) *BunStateRepository {
	return &BunStateRepository{db: db}
}

// Create inserts a new state row using the client-provided GUID.
func (r *BunStateRepository) Create(ctx context.Context, state *models.State) error {
	if err := state.ValidateForCreate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	now := time.Now()
	state.CreatedAt = now
	state.UpdatedAt = now

	_, err := r.db.NewInsert().Model(state).Exec(ctx)
	if err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("state with logic_id '%s' already exists", state.LogicID)
		}
		return fmt.Errorf("insert state: %w", err)
	}

	return nil
}

// GetByGUID fetches a state by its immutable GUID.
func (r *BunStateRepository) GetByGUID(ctx context.Context, guid string) (*models.State, error) {
	state := new(models.State)
	err := r.db.NewSelect().Model(state).Where("guid = ?", guid).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("state with guid '%s' not found", guid)
		}
		return nil, fmt.Errorf("query state: %w", err)
	}

	return state, nil
}

// GetByLogicID fetches a state via its human readable identifier.
func (r *BunStateRepository) GetByLogicID(ctx context.Context, logicID string) (*models.State, error) {
	state := new(models.State)
	err := r.db.NewSelect().Model(state).Where("logic_id = ?", logicID).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("state with logic_id '%s' not found", logicID)
		}
		return nil, fmt.Errorf("query state: %w", err)
	}

	return state, nil
}

// Update persists mutated state content and metadata.
// DEPRECATED: use UpdateContentAndUpsertOutputs for 003-ux-improvements-for/FR-027 compliance.
func (r *BunStateRepository) Update(ctx context.Context, state *models.State) error {
	state.UpdatedAt = time.Now()

	result, err := r.db.NewUpdate().
		Model(state).
		Column("state_content", "locked", "lock_info", "updated_at").
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update state: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("state with guid '%s' not found", state.GUID)
	}

	return nil
}

// UpdateContentAndUpsertOutputs atomically updates state content and output cache in one transaction.
// This ensures 003-ux-improvements-for/FR-027 compliance: cache and state are always consistent.
func (r *BunStateRepository) UpdateContentAndUpsertOutputs(ctx context.Context, guid string, content []byte, lockID string, serial int64, outputs []OutputKey) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// 1. Fetch state and validate lock
		state := new(models.State)
		err := tx.NewSelect().Model(state).Where("guid = ?", guid).Scan(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("state with guid '%s' not found", guid)
			}
			return fmt.Errorf("query state: %w", err)
		}

		// 2. Validate lock if state is locked
		if state.Locked {
			if lockID == "" || state.LockInfo == nil || lockID != state.LockInfo.ID {
				return fmt.Errorf("state is locked, cannot update")
			}
		}

		// 3. Update state content
		now := time.Now()
		result, err := tx.NewUpdate().
			Model((*models.State)(nil)).
			Set("state_content = ?", content).
			Set("updated_at = ?", now).
			Where("guid = ?", guid).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("update state content: %w", err)
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("state with guid '%s' not found", guid)
		}

		// 4. Delete old outputs with different serial (cache invalidation)
		_, err = tx.NewDelete().
			Model((*models.StateOutput)(nil)).
			Where("state_guid = ?", guid).
			Where("state_serial != ?", serial).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete stale outputs: %w", err)
		}

		// 5. Insert new outputs (if any)
		if len(outputs) > 0 {
			outputModels := make([]models.StateOutput, 0, len(outputs))
			for _, out := range outputs {
				outputModels = append(outputModels, models.StateOutput{
					StateGUID:   guid,
					OutputKey:   out.Key,
					Sensitive:   out.Sensitive,
					StateSerial: serial,
					CreatedAt:   now,
					UpdatedAt:   now,
				})
			}

			// Use ON CONFLICT DO UPDATE for idempotency
			_, err = tx.NewInsert().
				Model(&outputModels).
				On("CONFLICT (state_guid, output_key) DO UPDATE").
				Set("sensitive = EXCLUDED.sensitive").
				Set("state_serial = EXCLUDED.state_serial").
				Set("updated_at = EXCLUDED.updated_at").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("insert outputs: %w", err)
			}
		}

		return nil
	})
}

// List returns all states ordered from newest to oldest.
func (r *BunStateRepository) List(ctx context.Context) ([]models.State, error) {
	var states []models.State
	if err := r.db.NewSelect().Model(&states).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list states: %w", err)
	}

	if states == nil {
		states = []models.State{}
	}
	return states, nil
}

// Lock attempts to acquire an optimistic lock for the state.
func (r *BunStateRepository) Lock(ctx context.Context, guid string, lockInfo *models.LockInfo) error {
	result, err := r.db.NewUpdate().
		Model((*models.State)(nil)).
		Set("locked = ?", true).
		Set("lock_info = ?", lockInfo).
		Set("updated_at = ?", time.Now()).
		Where("guid = ?", guid).
		Where("locked = ?", false).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		current, lookupErr := r.GetByGUID(ctx, guid)
		if lookupErr != nil {
			return fmt.Errorf("state not found: %w", lookupErr)
		}
		if current.Locked && current.LockInfo != nil {
			return fmt.Errorf("state is already locked by %s", current.LockInfo.ID)
		}
		return fmt.Errorf("state with guid '%s' not found", guid)
	}

	return nil
}

// Unlock clears the lock metadata after verifying the current lock ID matches.
func (r *BunStateRepository) Unlock(ctx context.Context, guid string, lockID string) error {
	current, err := r.GetByGUID(ctx, guid)
	if err != nil {
		return err
	}

	if !current.Locked {
		return fmt.Errorf("state is not locked")
	}
	if current.LockInfo == nil || current.LockInfo.ID != lockID {
		return fmt.Errorf("lock ID mismatch: expected %s", current.LockInfo.ID)
	}

	result, err := r.db.NewUpdate().
		Model((*models.State)(nil)).
		Set("locked = ?", false).
		Set("lock_info = ?", nil).
		Set("updated_at = ?", time.Now()).
		Where("guid = ?", guid).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("state with guid '%s' not found", guid)
	}

	return nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "unique constraint") || strings.Contains(msg, "UNIQUE constraint") || strings.Contains(msg, "23505")
}
