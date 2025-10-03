package tfstate

import (
	"encoding/json"
	"fmt"
)

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
