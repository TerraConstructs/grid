package server

import (
	"context"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/validation"
)

// SchemaValidationJob manages schema validation for state outputs
// Runs SYNCHRONOUSLY (not fire-and-forget) to prevent race conditions with EdgeUpdateJob
type SchemaValidationJob struct {
	outputRepo repository.StateOutputRepository
	validator  validation.Validator
	timeout    time.Duration
}

// NewSchemaValidationJob creates a new validation job
func NewSchemaValidationJob(outputRepo repository.StateOutputRepository, validator validation.Validator, timeout time.Duration) *SchemaValidationJob {
	if timeout == 0 {
		timeout = 30 * time.Second // Default 30s timeout
	}

	return &SchemaValidationJob{
		outputRepo: outputRepo,
		validator:  validator,
		timeout:    timeout,
	}
}

// ValidateOutputs validates all outputs for a state
// Runs SYNCHRONOUSLY in the request path (blocks response by ~10-50ms)
// This guarantees validation_status is set before EdgeUpdateJob reads it
func (j *SchemaValidationJob) ValidateOutputs(ctx context.Context, stateGUID string, outputs map[string]interface{}) error {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()

	// Get all schemas for this state
	schemas, err := j.outputRepo.GetSchemasForState(timeoutCtx, stateGUID)
	if err != nil {
		// Log error but don't fail the request (validation is advisory)
		fmt.Printf("GetSchemasForState failed for state %s: %v\n", stateGUID, err)
		return nil // Non-blocking error
	}

	// If no schemas, mark all outputs as "not_validated" per FR-033
	if len(schemas) == 0 {
		return j.markOutputsAsNotValidated(timeoutCtx, stateGUID, outputs)
	}

	// Validate outputs that have schemas
	results, err := j.validator.ValidateOutputs(timeoutCtx, schemas, outputs)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("ValidateOutputs failed for state %s: %v\n", stateGUID, err)
		return nil // Non-blocking error
	}

	// Update validation status for each result
	for _, result := range results {
		err := j.outputRepo.UpdateValidationStatus(
			timeoutCtx,
			stateGUID,
			result.OutputKey,
			result.Status,
			result.ValidationError,
			result.ValidatedAt,
		)
		if err != nil {
			// Log error but continue with other outputs
			fmt.Printf("UpdateValidationStatus failed for output %s in state %s: %v\n", result.OutputKey, stateGUID, err)
		}
	}

	// Mark outputs without schemas as "not_validated"
	return j.markUnvalidatedOutputs(timeoutCtx, stateGUID, outputs, schemas)
}

// markOutputsAsNotValidated marks all outputs as "not_validated" when no schemas exist
func (j *SchemaValidationJob) markOutputsAsNotValidated(ctx context.Context, stateGUID string, outputs map[string]interface{}) error {
	now := time.Now()

	for outputKey := range outputs {
		err := j.outputRepo.UpdateValidationStatus(
			ctx,
			stateGUID,
			outputKey,
			"not_validated",
			nil, // No error for not_validated
			now,
		)
		if err != nil {
			fmt.Printf("UpdateValidationStatus (not_validated) failed for output %s in state %s: %v\n", outputKey, stateGUID, err)
		}
	}

	return nil
}

// markUnvalidatedOutputs marks outputs without schemas as "not_validated"
// Outputs that were validated are skipped (already updated in ValidateOutputs)
func (j *SchemaValidationJob) markUnvalidatedOutputs(ctx context.Context, stateGUID string, outputs map[string]interface{}, schemas map[string]string) error {
	now := time.Now()

	for outputKey := range outputs {
		// Skip outputs that have schemas (already validated)
		if _, hasSchema := schemas[outputKey]; hasSchema {
			continue
		}

		// Mark as "not_validated"
		err := j.outputRepo.UpdateValidationStatus(
			ctx,
			stateGUID,
			outputKey,
			"not_validated",
			nil, // No error for not_validated
			now,
		)
		if err != nil {
			fmt.Printf("UpdateValidationStatus (not_validated) failed for output %s in state %s: %v\n", outputKey, stateGUID, err)
		}
	}

	return nil
}
