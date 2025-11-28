package inference

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JLugagne/jsonschema-infer"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/state"
)

// inferrer implements state.SchemaInferrer using JLugagne/jsonschema-infer
type inferrer struct{}

// NewInferrer creates a new state.SchemaInferrer instance
func NewInferrer() state.SchemaInferrer {
	return &inferrer{}
}

// InferSchemas infers JSON Schema from output values for outputs that need schemas
func (i *inferrer) InferSchemas(ctx context.Context, stateGUID string, outputs map[string]interface{}, needsSchema []string) ([]state.InferredSchema, error) {
	var inferred []state.InferredSchema

	// Create a set of outputs that need schema inference
	needsSchemaSet := make(map[string]bool)
	for _, key := range needsSchema {
		needsSchemaSet[key] = true
	}

	// Infer schema for each output that needs one
	for outputKey, outputValue := range outputs {
		// Skip if this output doesn't need schema inference
		if !needsSchemaSet[outputKey] {
			continue
		}

		// Marshal output value to JSON for inference library
		valueJSON, err := json.Marshal(outputValue)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal output value for %s: %w", outputKey, err)
		}

		// Create generator and add sample
		generator := jsonschema.New()
		if err := generator.AddSample(string(valueJSON)); err != nil {
			return nil, fmt.Errorf("failed to add sample for %s: %w", outputKey, err)
		}

		// Generate schema (returns JSON string directly)
		schema, err := generator.Generate()
		if err != nil {
			return nil, fmt.Errorf("failed to generate schema for %s: %w", outputKey, err)
		}

		// schema is already a JSON string, use it directly (no marshaling needed)
		inferred = append(inferred, state.InferredSchema{
			OutputKey:  outputKey,
			SchemaJSON: string(schema),
		})
	}

	return inferred, nil
}
