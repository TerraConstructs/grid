package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// BunStateOutputRepository persists state outputs using Bun ORM against PostgreSQL.
type BunStateOutputRepository struct {
	db *bun.DB
}

// NewBunStateOutputRepository constructs a repository backed by Bun.
func NewBunStateOutputRepository(db *bun.DB) StateOutputRepository {
	return &BunStateOutputRepository{db: db}
}

// UpsertOutputs atomically replaces all outputs for a state.
// Deletes outputs with mismatched serial, then inserts new outputs.
func (r *BunStateOutputRepository) UpsertOutputs(ctx context.Context, stateGUID string, serial int64, outputs []OutputKey) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Delete old outputs with different serial (cache invalidation)
		_, err := tx.NewDelete().
			Model((*models.StateOutput)(nil)).
			Where("state_guid = ?", stateGUID).
			Where("state_serial != ?", serial).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete stale outputs: %w", err)
		}

		// Insert new outputs (skip if empty)
		if len(outputs) == 0 {
			return nil
		}

		now := time.Now()
		outputModels := make([]models.StateOutput, 0, len(outputs))
		for _, out := range outputs {
			outputModels = append(outputModels, models.StateOutput{
				StateGUID:   stateGUID,
				OutputKey:   out.Key,
				Sensitive:   out.Sensitive,
				StateSerial: serial,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
		}

		// Use ON CONFLICT DO UPDATE to handle race conditions
		// Note: We do NOT update schema_json here - schemas are managed separately via SetOutputSchema
		// This preserves user-defined schemas when Terraform state is uploaded
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

		return nil
	})
}

// GetOutputsByState returns all cached outputs for a state.
func (r *BunStateOutputRepository) GetOutputsByState(ctx context.Context, stateGUID string) ([]OutputKey, error) {
	var dbOutputs []models.StateOutput
	err := r.db.NewSelect().
		Model(&dbOutputs).
		Where("state_guid = ?", stateGUID).
		Order("output_key ASC"). // Deterministic ordering
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("query outputs for state %s: %w", stateGUID, err)
	}

	// Convert to repository OutputKey type
	outputs := make([]OutputKey, len(dbOutputs))
	for i, dbOut := range dbOutputs {
		outputs[i] = OutputKey{
			Key:        dbOut.OutputKey,
			Sensitive:  dbOut.Sensitive,
			SchemaJSON: dbOut.SchemaJSON,
		}
	}

	return outputs, nil
}

// SearchOutputsByKey finds all states with output matching key (exact match).
func (r *BunStateOutputRepository) SearchOutputsByKey(ctx context.Context, outputKey string) ([]StateOutputRef, error) {
	var dbOutputs []models.StateOutput
	err := r.db.NewSelect().
		Model(&dbOutputs).
		Where("output_key = ?", outputKey).
		Order("state_guid ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("search by output key '%s': %w", outputKey, err)
	}

	// Need to join with states table to get logic_id
	// Use a more efficient approach: fetch states in one query
	if len(dbOutputs) == 0 {
		return []StateOutputRef{}, nil
	}

	guids := make([]string, len(dbOutputs))
	for i, out := range dbOutputs {
		guids[i] = out.StateGUID
	}

	var states []models.State
	err = r.db.NewSelect().
		Model(&states).
		Where("guid IN (?)", bun.In(guids)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch states for outputs: %w", err)
	}

	// Build guid -> logic_id map
	guidToLogicID := make(map[string]string, len(states))
	for _, state := range states {
		guidToLogicID[state.GUID] = state.LogicID
	}

	// Build result
	refs := make([]StateOutputRef, 0, len(dbOutputs))
	for _, out := range dbOutputs {
		logicID, ok := guidToLogicID[out.StateGUID]
		if !ok {
			// State was deleted between queries, skip
			continue
		}
		refs = append(refs, StateOutputRef{
			StateGUID:    out.StateGUID,
			StateLogicID: logicID,
			OutputKey:    out.OutputKey,
			Sensitive:    out.Sensitive,
		})
	}

	return refs, nil
}

// DeleteOutputsByState removes all cached outputs for a state.
func (r *BunStateOutputRepository) DeleteOutputsByState(ctx context.Context, stateGUID string) error {
	_, err := r.db.NewDelete().
		Model((*models.StateOutput)(nil)).
		Where("state_guid = ?", stateGUID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete outputs for state %s: %w", stateGUID, err)
	}

	return nil
}

// SetOutputSchema sets or updates the JSON Schema for a specific state output.
// Creates the output record if it doesn't exist (with state_serial=0, sensitive=false).
func (r *BunStateOutputRepository) SetOutputSchema(ctx context.Context, stateGUID string, outputKey string, schemaJSON string) error {
	now := time.Now()

	// Use INSERT ... ON CONFLICT to upsert the schema
	output := models.StateOutput{
		StateGUID:   stateGUID,
		OutputKey:   outputKey,
		Sensitive:   false, // Default for schema-only outputs
		StateSerial: 0,     // Default serial for outputs that don't exist in state yet
		SchemaJSON:  &schemaJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := r.db.NewInsert().
		Model(&output).
		On("CONFLICT (state_guid, output_key) DO UPDATE").
		Set("schema_json = EXCLUDED.schema_json").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("set schema for output %s in state %s: %w", outputKey, stateGUID, err)
	}

	return nil
}

// GetOutputSchema retrieves the JSON Schema for a specific state output.
// Returns empty string if no schema has been set (not an error).
func (r *BunStateOutputRepository) GetOutputSchema(ctx context.Context, stateGUID string, outputKey string) (string, error) {
	var output models.StateOutput
	err := r.db.NewSelect().
		Model(&output).
		Where("state_guid = ?", stateGUID).
		Where("output_key = ?", outputKey).
		Scan(ctx)

	if err != nil {
		// sql.ErrNoRows is not an error - just means no schema set
		if err.Error() == "sql: no rows in result set" {
			return "", nil
		}
		return "", fmt.Errorf("get schema for output %s in state %s: %w", outputKey, stateGUID, err)
	}

	// Return empty string if schema is nil
	if output.SchemaJSON == nil {
		return "", nil
	}

	return *output.SchemaJSON, nil
}
