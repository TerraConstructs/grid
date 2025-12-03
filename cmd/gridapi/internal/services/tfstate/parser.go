package tfstate

import (
	"encoding/json"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// TFState represents the structure of Terraform state JSON
type TFState struct {
	Version          int                    `json:"version"`
	TerraformVersion string                 `json:"terraform_version,omitempty"`
	Serial           int64                  `json:"serial"`
	Outputs          map[string]OutputValue `json:"outputs"`
}

// TFOutputs represents the structure of Terraform state outputs
type TFOutputs struct {
	Outputs map[string]OutputValue `json:"outputs"`
}

// OutputValue represents a single Terraform output value
type OutputValue struct {
	Value     interface{} `json:"value"`
	Sensitive bool        `json:"sensitive,omitempty"`
}

// ParseOutputs extracts output values from Terraform state JSON
func ParseOutputs(tfstateJSON []byte) (map[string]interface{}, error) {
	if len(tfstateJSON) == 0 {
		return make(map[string]interface{}), nil
	}

	var state TFOutputs
	if err := json.Unmarshal(tfstateJSON, &state); err != nil {
		return nil, fmt.Errorf("failed to parse tfstate JSON: %w", err)
	}

	outputs := make(map[string]interface{}, len(state.Outputs))
	for k, v := range state.Outputs {
		outputs[k] = v.Value
	}

	return outputs, nil
}

// ParseOutputKeys extracts output keys with sensitive flags from Terraform state JSON
func ParseOutputKeys(tfstateJSON []byte) ([]repository.OutputKey, error) {
	if len(tfstateJSON) == 0 {
		return []repository.OutputKey{}, nil
	}

	var state TFOutputs
	if err := json.Unmarshal(tfstateJSON, &state); err != nil {
		return nil, fmt.Errorf("failed to parse tfstate JSON: %w", err)
	}

	outputs := make([]repository.OutputKey, 0, len(state.Outputs))
	for k, v := range state.Outputs {
		outputs = append(outputs, repository.OutputKey{
			Key:       k,
			Sensitive: v.Sensitive,
		})
	}

	return outputs, nil
}

// ParseSerial extracts the serial number from Terraform state JSON
func ParseSerial(tfstateJSON []byte) (int64, error) {
	if len(tfstateJSON) == 0 {
		return 0, fmt.Errorf("empty state JSON")
	}

	var state TFState
	if err := json.Unmarshal(tfstateJSON, &state); err != nil {
		return 0, fmt.Errorf("failed to parse tfstate JSON: %w", err)
	}

	return state.Serial, nil
}

// ParsedState represents the parsed Terraform state with serial, keys, and values
type ParsedState struct {
	Serial int64
	Keys   []repository.OutputKey
	Values map[string]interface{}
}

// ParseState parses Terraform state JSON once and returns serial, output keys, and output values
// This avoids multiple parsing passes for the same state JSON
func ParseState(tfstateJSON []byte) (*ParsedState, error) {
	if len(tfstateJSON) == 0 {
		return &ParsedState{
			Serial: 0,
			Keys:   []repository.OutputKey{},
			Values: make(map[string]interface{}),
		}, nil
	}

	var state TFState
	if err := json.Unmarshal(tfstateJSON, &state); err != nil {
		return nil, fmt.Errorf("failed to parse tfstate JSON: %w", err)
	}

	// Extract keys with sensitive flags
	keys := make([]repository.OutputKey, 0, len(state.Outputs))
	values := make(map[string]interface{}, len(state.Outputs))
	for k, v := range state.Outputs {
		keys = append(keys, repository.OutputKey{
			Key:       k,
			Sensitive: v.Sensitive,
		})
		values[k] = v.Value
	}

	return &ParsedState{
		Serial: state.Serial,
		Keys:   keys,
		Values: values,
	}, nil
}
