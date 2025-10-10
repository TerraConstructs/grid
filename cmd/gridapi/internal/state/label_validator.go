package state

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// Label key format: lowercase start, then alphanumeric + underscore + forward-slash, ≤32 chars
// Compatible with go-bexpr default Identifier grammar
var labelKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_/]{0,31}$`)

// LabelValidator validates labels against policy constraints.
type LabelValidator struct {
	policy *models.PolicyDefinition
}

// NewLabelValidator constructs a validator with the given policy.
func NewLabelValidator(policy *models.PolicyDefinition) *LabelValidator {
	return &LabelValidator{policy: policy}
}

// Validate checks labels against policy constraints.
// T031: Implements key format regex, enum validation, reserved prefixes, size limits.
func (v *LabelValidator) Validate(labels models.LabelMap) error {
	if v.policy == nil {
		// No policy: apply basic validation only
		return v.validateBasic(labels)
	}

	// Check max keys
	if len(labels) > v.policy.MaxKeys {
		return fmt.Errorf("label count %d exceeds max_keys %d", len(labels), v.policy.MaxKeys)
	}

	for key, value := range labels {
		// 1. Validate key format
		if !labelKeyRE.MatchString(key) {
			return fmt.Errorf("label key '%s' does not match required format: must start with lowercase letter, contain only lowercase alphanumeric, underscore, or forward-slash, and be ≤32 characters", key)
		}

		// 2. Check reserved prefixes
		for _, prefix := range v.policy.ReservedPrefixes {
			if strings.HasPrefix(key, prefix) {
				return fmt.Errorf("label key '%s' uses reserved prefix '%s'", key, prefix)
			}
		}

		// 3. Check allowed keys (if policy defines them)
		if len(v.policy.AllowedKeys) > 0 {
			if _, allowed := v.policy.AllowedKeys[key]; !allowed {
				return fmt.Errorf("label key '%s' is not in allowed_keys policy", key)
			}
		}

		// 4. Validate value type and constraints
		switch val := value.(type) {
		case string:
			if len(val) > v.policy.MaxValueLen {
				return fmt.Errorf("label '%s' string value length %d exceeds max_value_len %d", key, len(val), v.policy.MaxValueLen)
			}

			// Check enum constraint (if defined for this key)
			if allowedValues, hasEnum := v.policy.AllowedValues[key]; hasEnum {
				if !contains(allowedValues, val) {
					return fmt.Errorf("label '%s' value '%s' not in allowed enum values %v", key, val, allowedValues)
				}
			}

		case float64:
			// Numbers are always valid (no max constraint for now)

		case bool:
			// Booleans are always valid

		default:
			return fmt.Errorf("label '%s' has unsupported value type %T (only string, number, bool allowed)", key, value)
		}
	}

	return nil
}

// validateBasic applies minimal validation when no policy is set.
func (v *LabelValidator) validateBasic(labels models.LabelMap) error {
	for key, value := range labels {
		// Basic key format check
		if !labelKeyRE.MatchString(key) {
			return fmt.Errorf("label key '%s' does not match required format", key)
		}

		// Basic type check
		switch value.(type) {
		case string, float64, bool:
			// Valid types
		default:
			return fmt.Errorf("label '%s' has unsupported value type %T", key, value)
		}
	}

	return nil
}

// contains checks if a slice contains a string.
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
