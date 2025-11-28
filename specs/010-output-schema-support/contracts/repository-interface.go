// Repository Interface Extensions for Output Schema Support - Phase 2
// File: cmd/gridapi/internal/repository/interface.go
//
// This file shows the complete interface after Phase 2 changes

package repository

import (
	"context"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// OutputKey represents a Terraform output name and metadata.
// Extended in Phase 2 to include schema source and validation fields.
type OutputKey struct {
	Key              string
	Sensitive        bool
	SchemaJSON       *string    // Optional JSON Schema definition
	SchemaSource     *string    // "manual" | "inferred" | nil
	ValidationStatus *string    // "valid" | "invalid" | "error" | nil
	ValidationError  *string    // Error message if status != "valid"
	ValidatedAt      *time.Time // Last validation timestamp
}

// ValidationResult represents the outcome of validating an output against its schema.
type ValidationResult struct {
	Status      string    // "valid", "invalid", "error"
	Error       *string   // Error message with path information
	ValidatedAt time.Time
}

// OutputKeyWithValidation pairs an output key with its validation result.
// Used for atomic upsert of outputs with validation status.
type OutputKeyWithValidation struct {
	OutputKey
	Validation *ValidationResult // nil if no schema to validate against
}

// InferredSchema represents a schema generated from output data.
type InferredSchema struct {
	OutputKey  string
	SchemaJSON string
}

// StateOutputRepository exposes persistence operations for cached Terraform outputs.
type StateOutputRepository interface {
	// =========================================================================
	// Existing Methods (Phase 1 - unchanged signatures)
	// =========================================================================

	// UpsertOutputs atomically replaces all outputs for a state.
	// Preserves schema_json, schema_source, validation_* fields.
	// Deletes old outputs where state_serial != serial, inserts new ones.
	UpsertOutputs(ctx context.Context, stateGUID string, serial int64, outputs []OutputKey) error

	// GetOutputsByState returns all cached outputs for a state.
	// Returns empty slice if no outputs exist (not an error).
	// Includes all schema and validation metadata in response.
	GetOutputsByState(ctx context.Context, stateGUID string) ([]OutputKey, error)

	// SearchOutputsByKey finds all states with output matching key (exact match).
	// Used for cross-state dependency discovery.
	SearchOutputsByKey(ctx context.Context, outputKey string) ([]StateOutputRef, error)

	// DeleteOutputsByState removes all cached outputs for a state.
	DeleteOutputsByState(ctx context.Context, stateGUID string) error

	// SetOutputSchema sets or updates the JSON Schema for a specific state output.
	// Creates the output record if it doesn't exist (with state_serial=0, sensitive=false).
	// Sets schema_source to "manual".
	SetOutputSchema(ctx context.Context, stateGUID string, outputKey string, schemaJSON string) error

	// GetOutputSchema retrieves the JSON Schema for a specific state output.
	// Returns empty string if no schema has been set (not an error).
	GetOutputSchema(ctx context.Context, stateGUID string, outputKey string) (string, error)

	// =========================================================================
	// New Methods (Phase 2A - Schema Inference)
	// =========================================================================

	// SetOutputSchemaWithSource sets schema with explicit source indicator.
	// source must be "manual" or "inferred".
	// Creates output record if it doesn't exist.
	SetOutputSchemaWithSource(ctx context.Context, stateGUID, outputKey, schemaJSON, source string) error

	// GetOutputsWithoutSchema returns output keys that have no schema defined.
	// Used to determine which outputs need schema inference.
	// Only returns outputs that exist in the state (state_serial > 0).
	GetOutputsWithoutSchema(ctx context.Context, stateGUID string) ([]string, error)

	// =========================================================================
	// New Methods (Phase 2B - Schema Validation)
	// =========================================================================

	// UpsertOutputsWithValidation atomically updates outputs and validation results.
	// Used after state upload + validation completes.
	// Preserves schema_json and schema_source (only updates validation_* fields).
	UpsertOutputsWithValidation(ctx context.Context, stateGUID string, serial int64, outputs []OutputKeyWithValidation) error

	// UpdateValidationStatus updates validation result for a single output.
	// Used by background validation job.
	UpdateValidationStatus(ctx context.Context, stateGUID, outputKey string, result ValidationResult) error

	// GetSchemasForState returns all output schemas for a state.
	// Returns map of outputKey -> schemaJSON.
	// Only includes outputs with non-empty schema_json.
	// Used by validator to fetch schemas for all outputs at once.
	GetSchemasForState(ctx context.Context, stateGUID string) (map[string]string, error)
}

// StateOutputRef represents a state reference with an output key.
type StateOutputRef struct {
	StateGUID    string
	StateLogicID string
	OutputKey    string
	Sensitive    bool
}
