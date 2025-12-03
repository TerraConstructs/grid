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
		// Build set of new output keys for quick lookup
		newOutputKeys := make(map[string]bool, len(outputs))
		for _, out := range outputs {
			newOutputKeys[out.Key] = true
		}

		// Fetch ALL existing outputs for this state (not just ones with schemas)
		var existingOutputs []models.StateOutput
		err := tx.NewSelect().
			Model(&existingOutputs).
			Where("state_guid = ?", stateGUID).
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("fetch existing outputs: %w", err)
		}

		// Build map of output_key -> full existing output data
		existingByKey := make(map[string]*models.StateOutput, len(existingOutputs))
		for i := range existingOutputs {
			existingByKey[existingOutputs[i].OutputKey] = &existingOutputs[i]
		}

		// Delete outputs that no longer exist in the new upload
		// Only delete inferred schemas; manual schemas are retained as orphans
		for _, existing := range existingOutputs {
			if !newOutputKeys[existing.OutputKey] {
				// Output no longer exists in Terraform state
				if existing.SchemaSource == nil || *existing.SchemaSource == "inferred" {
					// Delete inferred/no-schema outputs that were removed
					_, err = tx.NewDelete().
						Model((*models.StateOutput)(nil)).
						Where("state_guid = ?", stateGUID).
						Where("output_key = ?", existing.OutputKey).
						Exec(ctx)
					if err != nil {
						return fmt.Errorf("delete removed output %s: %w", existing.OutputKey, err)
					}
				}
				// Manual schemas are kept as orphans (output may return later)
			}
		}

		// Skip if no outputs to upsert
		if len(outputs) == 0 {
			return nil
		}

		now := time.Now()
		outputModels := make([]models.StateOutput, 0, len(outputs))
		for _, out := range outputs {
			model := models.StateOutput{
				StateGUID:   stateGUID,
				OutputKey:   out.Key,
				Sensitive:   out.Sensitive,
				StateSerial: serial,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			// Preserve existing schema metadata if output already exists
			// Fix for grid-58bb: Preserve ALL schema metadata fields
			if existing, ok := existingByKey[out.Key]; ok {
				model.SchemaJSON = existing.SchemaJSON
				model.SchemaSource = existing.SchemaSource
				model.ValidationStatus = existing.ValidationStatus
				model.ValidationError = existing.ValidationError
				model.ValidatedAt = existing.ValidatedAt
			}
			outputModels = append(outputModels, model)
		}

		// Use ON CONFLICT DO UPDATE to upsert
		// This handles both new outputs and existing outputs with preserved metadata
		_, err = tx.NewInsert().
			Model(&outputModels).
			On("CONFLICT (state_guid, output_key) DO UPDATE").
			Set("sensitive = EXCLUDED.sensitive").
			Set("state_serial = EXCLUDED.state_serial").
			Set("updated_at = EXCLUDED.updated_at").
			Set("schema_json = EXCLUDED.schema_json").
			Set("schema_source = EXCLUDED.schema_source").
			Set("validation_status = EXCLUDED.validation_status").
			Set("validation_error = EXCLUDED.validation_error").
			Set("validated_at = EXCLUDED.validated_at").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("upsert outputs: %w", err)
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
			Key:              dbOut.OutputKey,
			Sensitive:        dbOut.Sensitive,
			StateSerial:      dbOut.StateSerial,
			SchemaJSON:       dbOut.SchemaJSON,
			SchemaSource:     dbOut.SchemaSource,
			ValidationStatus: dbOut.ValidationStatus,
			ValidationError:  dbOut.ValidationError,
			ValidatedAt:      dbOut.ValidatedAt,
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
// Always sets schema_source to "manual" since this is an explicit SetOutputSchema call.
func (r *BunStateOutputRepository) SetOutputSchema(ctx context.Context, stateGUID string, outputKey string, schemaJSON string) error {
	source := "manual"
	// Use -1 to skip serial check (manual schemas always write)
	return r.SetOutputSchemaWithSource(ctx, stateGUID, outputKey, schemaJSON, source, -1)
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

// SetOutputSchemaWithSource sets or updates the JSON Schema with source tracking.
// source must be "manual" or "inferred".
// Creates the output record if it doesn't exist (with state_serial=0, sensitive=false).
// expectedSerial: For inferred schemas, verifies output still exists at this serial before writing.
//
//	Use -1 for manual schemas to skip serial check (always write).
func (r *BunStateOutputRepository) SetOutputSchemaWithSource(ctx context.Context, stateGUID, outputKey, schemaJSON, source string, expectedSerial int64) error {
	now := time.Now()

	// Serial check for inferred schemas to prevent resurrection
	// (skip check if expectedSerial == -1, used for manual schemas)
	if expectedSerial >= 0 {
		var existingOutput models.StateOutput
		err := r.db.NewSelect().
			Model(&existingOutput).
			Where("state_guid = ?", stateGUID).
			Where("output_key = ?", outputKey).
			Where("state_serial = ?", expectedSerial).
			Scan(ctx)

		if err != nil {
			// Output missing or serial changed, skip write (silent no-op)
			// This prevents resurrecting outputs removed by newer state uploads
			return nil
		}
	}

	// Use INSERT ... ON CONFLICT to upsert the schema with source
	output := models.StateOutput{
		StateGUID:    stateGUID,
		OutputKey:    outputKey,
		Sensitive:    false, // Default for schema-only outputs
		StateSerial:  0,     // Default serial for outputs that don't exist in state yet
		SchemaJSON:   &schemaJSON,
		SchemaSource: &source,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err := r.db.NewInsert().
		Model(&output).
		On("CONFLICT (state_guid, output_key) DO UPDATE").
		Set("schema_json = EXCLUDED.schema_json").
		Set("schema_source = EXCLUDED.schema_source").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("set schema with source %s for output %s in state %s: %w", source, outputKey, stateGUID, err)
	}

	return nil
}

// GetOutputsWithoutSchema returns output keys that don't have a schema set.
// Used by inference service to determine which outputs need schema generation.
// Returns empty slice if all outputs have schemas (not an error).
func (r *BunStateOutputRepository) GetOutputsWithoutSchema(ctx context.Context, stateGUID string) ([]string, error) {
	var outputs []models.StateOutput
	err := r.db.NewSelect().
		Model(&outputs).
		Column("output_key").
		Where("state_guid = ?", stateGUID).
		Where("schema_json IS NULL").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("get outputs without schema for state %s: %w", stateGUID, err)
	}

	// Extract output keys
	keys := make([]string, len(outputs))
	for i, output := range outputs {
		keys[i] = output.OutputKey
	}

	return keys, nil
}

// GetSchemasForState returns all output schemas for a state (for validation).
// Returns map of outputKey -> schemaJSON for outputs that have schemas.
// Outputs without schemas are not included in the map.
func (r *BunStateOutputRepository) GetSchemasForState(ctx context.Context, stateGUID string) (map[string]string, error) {
	var outputs []models.StateOutput
	err := r.db.NewSelect().
		Model(&outputs).
		Column("output_key", "schema_json").
		Where("state_guid = ?", stateGUID).
		Where("schema_json IS NOT NULL").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("get schemas for state %s: %w", stateGUID, err)
	}

	// Build map of output key -> schema JSON
	schemas := make(map[string]string, len(outputs))
	for _, output := range outputs {
		if output.SchemaJSON != nil {
			schemas[output.OutputKey] = *output.SchemaJSON
		}
	}

	return schemas, nil
}

// UpdateValidationStatus updates the validation status for a specific output.
// Sets validation_status, validation_error, and validated_at columns.
// validationError can be nil for "valid" or "not_validated" statuses.
func (r *BunStateOutputRepository) UpdateValidationStatus(ctx context.Context, stateGUID, outputKey, status string, validationError *string, validatedAt time.Time) error {
	_, err := r.db.NewUpdate().
		Model((*models.StateOutput)(nil)).
		Set("validation_status = ?", status).
		Set("validation_error = ?", validationError).
		Set("validated_at = ?", validatedAt).
		Set("updated_at = ?", validatedAt).
		Where("state_guid = ?", stateGUID).
		Where("output_key = ?", outputKey).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("update validation status for output %s in state %s: %w", outputKey, stateGUID, err)
	}

	return nil
}
