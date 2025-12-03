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
	OutputKey       string
	Status          string  // "valid", "invalid", "error"
	ValidationError *string // JSON path and error details (if invalid or error)
	ValidatedAt     time.Time
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

// formatValidationError traverses the ValidationError tree to extract specific messages
// without parsing string outputs or scrubbing file paths.
// Uses the structured fields from jsonschema.ValidationError directly.
func (v *SchemaValidator) formatValidationError(err error) string {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return err.Error()
	}

	// 1. Flatten the error tree to get actual validation failures (leaves)
	leafErrors := getLeafErrors(ve)

	// 2. Format each error into a readable string
	var messages []string
	for _, leaf := range leafErrors {
		// Convert InstanceLocation ([]string path segments) to Dot Notation ("$.a.b")
		path := jsonPointerToDot(leaf.InstanceLocation)

		// Get the full, formatted error message from the library.
		// In jsonschema v6, leaf.Message only contains raw arguments (e.g., "150 10").
		// leaf.Error() returns the full formatted string with proper English descriptions.
		fullMsg := leaf.Error()

		// Clean the message: remove the "jsonschema: " prefix and the schema location.
		// The format is typically: "jsonschema: {Location}: {Message}" or "{Location}: {Message}"
		msg := strings.TrimPrefix(fullMsg, "jsonschema: ")

		// Split by the first ": " to separate the schema path from the actual message.
		// This robustly handles different schema URL formats (file://, http://, or simple filenames).
		if i := strings.Index(msg, ": "); i != -1 {
			msg = msg[i+2:]
		}

		messages = append(messages, fmt.Sprintf("at '%s': %s", path, msg))
	}

	// 3. Join multiple errors or just take the first one
	// Using a semicolon to separate multiple validation errors
	finalMsg := strings.Join(messages, "; ")

	// Truncate if excessively long
	if len(finalMsg) > 200 {
		return finalMsg[:200] + "... (truncated)"
	}
	return finalMsg
}

// getLeafErrors recursively finds the errors that resulted in the validation failure.
// jsonschema returns a tree; "Causes" hold the detailed errors.
// A leaf error is one with no causes (the actual validation failure).
func getLeafErrors(ve *jsonschema.ValidationError) []*jsonschema.ValidationError {
	// If there are no causes, this is a leaf error (the actual failure)
	if len(ve.Causes) == 0 {
		return []*jsonschema.ValidationError{ve}
	}

	var leaves []*jsonschema.ValidationError
	for _, cause := range ve.Causes {
		leaves = append(leaves, getLeafErrors(cause)...)
	}
	return leaves
}

// jsonPointerToDot converts a JSON Pointer path (InstanceLocation []string segments) to Dot notation.
// Example: []string{"users", "0", "name"} -> "$.users.0.name"
// Example: []string{} or []string{""} -> "$"
func jsonPointerToDot(pathSegments []string) string {
	if len(pathSegments) == 0 {
		return "$"
	}

	// Filter out empty segments (InstanceLocation sometimes includes empty strings)
	var nonEmptySegments []string
	for _, segment := range pathSegments {
		if segment != "" {
			nonEmptySegments = append(nonEmptySegments, segment)
		}
	}

	if len(nonEmptySegments) == 0 {
		return "$"
	}

	// Join segments with dots
	return "$." + strings.Join(nonEmptySegments, ".")
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
