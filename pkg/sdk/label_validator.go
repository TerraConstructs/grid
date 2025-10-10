package sdk

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var labelKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_/]{0,31}$`)

// PolicyDefinition mirrors the server-side structure for label validation rules.
type PolicyDefinition struct {
	AllowedKeys      map[string]struct{} `json:"allowed_keys,omitempty"`
	AllowedValues    map[string][]string `json:"allowed_values,omitempty"`
	ReservedPrefixes []string            `json:"reserved_prefixes,omitempty"`
	MaxKeys          int                 `json:"max_keys"`
	MaxValueLen      int                 `json:"max_value_len"`
}

// ParsePolicyDefinition decodes JSON policy into a PolicyDefinition.
func ParsePolicyDefinition(policyJSON string) (*PolicyDefinition, error) {
	if strings.TrimSpace(policyJSON) == "" {
		return nil, fmt.Errorf("policy JSON is empty")
	}

	var definition PolicyDefinition
	if err := json.Unmarshal([]byte(policyJSON), &definition); err != nil {
		return nil, err
	}

	if definition.MaxKeys == 0 {
		definition.MaxKeys = 32
	}
	if definition.MaxValueLen == 0 {
		definition.MaxValueLen = 256
	}

	return &definition, nil
}

// LabelValidator validates labels against optional policy constraints.
type LabelValidator struct {
	policy *PolicyDefinition
}

// NewLabelValidator constructs a validator (nil policy enables basic validation only).
func NewLabelValidator(policy *PolicyDefinition) *LabelValidator {
	return &LabelValidator{policy: policy}
}

// Validate checks labels against policy constraints.
func (v *LabelValidator) Validate(labels LabelMap) error {
	if v.policy == nil {
		return validateBasic(labels)
	}

	policy := v.policy
	if len(labels) > policy.MaxKeys {
		return fmt.Errorf("label count %d exceeds max_keys %d", len(labels), policy.MaxKeys)
	}

	for key, value := range labels {
		if !labelKeyRE.MatchString(key) {
			return fmt.Errorf("label key '%s' does not match required format", key)
		}

		for _, prefix := range policy.ReservedPrefixes {
			if strings.HasPrefix(key, prefix) {
				return fmt.Errorf("label key '%s' uses reserved prefix '%s'", key, prefix)
			}
		}

		if len(policy.AllowedKeys) > 0 {
			if _, ok := policy.AllowedKeys[key]; !ok {
				return fmt.Errorf("label key '%s' is not in allowed_keys policy", key)
			}
		}

		switch val := value.(type) {
		case string:
			if len(val) > policy.MaxValueLen {
				return fmt.Errorf("label '%s' string value length %d exceeds max_value_len %d", key, len(val), policy.MaxValueLen)
			}
			if allowedValues, ok := policy.AllowedValues[key]; ok && !containsValue(allowedValues, val) {
				return fmt.Errorf("label '%s' value '%s' not in allowed enum values %v", key, val, allowedValues)
			}
		case float64:
			// numbers allowed
		case bool:
			// booleans allowed
		default:
			return fmt.Errorf("label '%s' has unsupported value type %T (only string, number, bool allowed)", key, value)
		}
	}

	return nil
}

func validateBasic(labels LabelMap) error {
	for key, value := range labels {
		if !labelKeyRE.MatchString(key) {
			return fmt.Errorf("label key '%s' does not match required format", key)
		}
		switch value.(type) {
		case string, float64, bool:
			// ok
		default:
			return fmt.Errorf("label '%s' has unsupported value type %T", key, value)
		}
	}
	return nil
}

func containsValue(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
