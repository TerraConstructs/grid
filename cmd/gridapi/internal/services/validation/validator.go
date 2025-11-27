package validation

import (
	"context"
	"fmt"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidationResult represents the result of validating an output against its schema
type ValidationResult struct {
	OutputKey        string
	Status           string  // "valid", "invalid", "error"
	ValidationError  *string // JSON path and error details (if invalid or error)
	ValidatedAt      time.Time
}

// Validator validates output values against JSON schemas
type Validator interface {
	// ValidateOutputs validates multiple outputs against their schemas
	// Returns results only for outputs that have schemas (skips outputs without schemas per FR-033)
	ValidateOutputs(ctx context.Context, schemas map[string]string, outputs map[string]interface{}) ([]ValidationResult, error)
}

// SchemaValidator implements Validator using santhosh-tekuri/jsonschema/v6
type SchemaValidator struct {
	schemaCache *lru.Cache[string, *jsonschema.Schema]
	compiler    *jsonschema.Compiler
}

// NewSchemaValidator creates a new validator with LRU caching for compiled schemas
func NewSchemaValidator(cacheSize int) (*SchemaValidator, error) {
	cache, err := lru.New[string, *jsonschema.Schema](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("create schema cache: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	// Use JSON Schema Draft 7 as default
	compiler.DefaultDraft(jsonschema.Draft7)

	return &SchemaValidator{
		schemaCache: cache,
		compiler:    compiler,
	}, nil
}

// ValidateOutputs validates all outputs that have schemas
// Outputs without schemas are skipped (caller should set validation_status="not_validated")
func (v *SchemaValidator) ValidateOutputs(ctx context.Context, schemas map[string]string, outputs map[string]interface{}) ([]ValidationResult, error) {
	var results []ValidationResult
	now := time.Now()

	for outputKey, schemaJSON := range schemas {
		// Get output value (may not exist if schema was pre-declared)
		outputValue, exists := outputs[outputKey]
		if !exists {
			// Schema exists but output doesn't - skip validation
			// This is normal for pre-declared schemas (state_serial=0, schema_source=manual)
			continue
		}

		// Validate the output
		result := v.validateOutput(ctx, outputKey, schemaJSON, outputValue, now)
		results = append(results, result)
	}

	return results, nil
}

// validateOutput validates a single output against its schema
func (v *SchemaValidator) validateOutput(ctx context.Context, outputKey, schemaJSON string, outputValue interface{}, now time.Time) ValidationResult {
	// Try to get compiled schema from cache
	cacheKey := schemaJSON // Use schema JSON as cache key (schemas are unique)
	cachedSchema, found := v.schemaCache.Get(cacheKey)

	var schema *jsonschema.Schema
	var err error

	if found {
		schema = cachedSchema
	} else {
		// Compile schema (not in cache)
		schema, err = v.compileSchema(schemaJSON)
		if err != nil {
			// System error: malformed schema
			errMsg := fmt.Sprintf("Schema compilation failed: %v", err)
			return ValidationResult{
				OutputKey:       outputKey,
				Status:          "error",
				ValidationError: &errMsg,
				ValidatedAt:     now,
			}
		}

		// Cache the compiled schema
		v.schemaCache.Add(cacheKey, schema)
	}

	// Validate output value against schema
	err = schema.Validate(outputValue)
	if err != nil {
		// Data error: output violates schema
		errMsg := v.formatValidationError(err)
		return ValidationResult{
			OutputKey:       outputKey,
			Status:          "invalid",
			ValidationError: &errMsg,
			ValidatedAt:     now,
		}
	}

	// Validation passed
	return ValidationResult{
		OutputKey:       outputKey,
		Status:          "valid",
		ValidationError: nil,
		ValidatedAt:     now,
	}
}

// compileSchema compiles a JSON schema string into a schema object
func (v *SchemaValidator) compileSchema(schemaJSON string) (*jsonschema.Schema, error) {
	// Parse schema JSON
	parsed, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse schema JSON: %w", err)
	}

	// Create a new compiler for this schema (to avoid conflicts)
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)

	// Add schema as a resource
	schemaURL := "schema.json"
	if err := compiler.AddResource(schemaURL, parsed); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}

	// Compile the schema
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	return schema, nil
}

// formatValidationError formats a validation error into a structured message per SC-006 and FR-035.
// Includes: JSON path, error description (with truncation for long messages).
// Example: "validation failed at '$.vpc_id': does not match pattern '^vpc-[a-f0-9]{8,17}$'"
func (v *SchemaValidator) formatValidationError(err error) string {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		// Not a validation error, return generic message
		return err.Error()
	}

	// Build JSON path from InstanceLocation (e.g., ["", "vpc_id"] -> "$.vpc_id")
	var path string
	if len(ve.InstanceLocation) > 0 {
		// Filter out empty strings and build path
		var parts []string
		for _, part := range ve.InstanceLocation {
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			path = "$." + strings.Join(parts, ".")
		} else {
			path = "$"
		}
	} else {
		path = "$"
	}

	// Get the error message from the library (includes constraint details)
	errorMsg := ve.Error()

	// Truncate if message is excessively long (>200 chars)
	if len(errorMsg) > 200 {
		errorMsg = errorMsg[:200] + "... (truncated)"
	}

	// Format structured error with path
	return fmt.Sprintf("validation failed at '%s': %s", path, errorMsg)
}

// InvalidateCache removes a schema from the cache
// Call this when a schema is updated via SetOutputSchema
func (v *SchemaValidator) InvalidateCache(schemaJSON string) {
	v.schemaCache.Remove(schemaJSON)
}

// GetCacheSize returns cache size for monitoring
func (v *SchemaValidator) GetCacheSize() int {
	return v.schemaCache.Len()
}
