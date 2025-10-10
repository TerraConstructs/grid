package state

import (
	"encoding/json"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// PolicyValidator validates policy structure before persistence.
// T034a: Implements FR-029 validation (malformed JSON, invalid schema).
type PolicyValidator struct{}

// NewPolicyValidator constructs a policy validator.
func NewPolicyValidator() *PolicyValidator {
	return &PolicyValidator{}
}

// ValidatePolicyStructure checks policy JSON structure and constraints.
// FR-029: Rejects malformed JSON and invalid schema (missing fields, wrong types, bad limits).
func (v *PolicyValidator) ValidatePolicyStructure(policyJSON []byte) (*models.PolicyDefinition, error) {
	if len(policyJSON) == 0 {
		return nil, fmt.Errorf("policy JSON cannot be empty")
	}

	// 1. Check valid JSON
	var policy models.PolicyDefinition
	if err := json.Unmarshal(policyJSON, &policy); err != nil {
		return nil, fmt.Errorf("malformed JSON: %w", err)
	}

	// 2. Validate required fields and sensible defaults
	if policy.MaxKeys == 0 {
		policy.MaxKeys = 32 // default
	}
	if policy.MaxValueLen == 0 {
		policy.MaxValueLen = 256 // default
	}

	// 3. Check sensible limits
	if policy.MaxKeys < 1 || policy.MaxKeys > 1000 {
		return nil, fmt.Errorf("max_keys must be between 1 and 1000, got %d", policy.MaxKeys)
	}

	if policy.MaxValueLen < 1 || policy.MaxValueLen > 10000 {
		return nil, fmt.Errorf("max_value_len must be between 1 and 10000, got %d", policy.MaxValueLen)
	}

	// 4. Validate allowed_keys structure (if present)
	if policy.AllowedKeys != nil {
		for key := range policy.AllowedKeys {
			if key == "" {
				return nil, fmt.Errorf("allowed_keys contains empty key")
			}
			// Validate key format against label regex
			if !labelKeyRE.MatchString(key) {
				return nil, fmt.Errorf("allowed_keys contains invalid key '%s': must match label key format", key)
			}
		}
	}

	// 5. Validate allowed_values structure (if present)
	if policy.AllowedValues != nil {
		for key, values := range policy.AllowedValues {
			if key == "" {
				return nil, fmt.Errorf("allowed_values contains empty key")
			}
			// Validate key format
			if !labelKeyRE.MatchString(key) {
				return nil, fmt.Errorf("allowed_values key '%s' does not match label key format", key)
			}

			// Check enum values are non-empty and reasonable
			if len(values) == 0 {
				return nil, fmt.Errorf("allowed_values for key '%s' is empty (omit key if any value allowed)", key)
			}

			if len(values) > 1000 {
				return nil, fmt.Errorf("allowed_values for key '%s' exceeds maximum of 1000 values", key)
			}

			// Check individual values
			for _, val := range values {
				if val == "" {
					return nil, fmt.Errorf("allowed_values for key '%s' contains empty string", key)
				}
				if len(val) > policy.MaxValueLen {
					return nil, fmt.Errorf("allowed_values for key '%s' contains value exceeding max_value_len (%d): '%s'", key, policy.MaxValueLen, val)
				}
			}
		}
	}

	// 6. Validate reserved_prefixes (if present)
	if len(policy.ReservedPrefixes) > 0 {
		seen := make(map[string]bool)
		for _, prefix := range policy.ReservedPrefixes {
			if prefix == "" {
				return nil, fmt.Errorf("reserved_prefixes contains empty string")
			}
			if seen[prefix] {
				return nil, fmt.Errorf("reserved_prefixes contains duplicate: '%s'", prefix)
			}
			seen[prefix] = true

			// Reserved prefixes should end with / or : for clarity
			// (This is a soft warning - not enforced strictly)
		}

		if len(policy.ReservedPrefixes) > 100 {
			return nil, fmt.Errorf("reserved_prefixes exceeds maximum of 100 prefixes")
		}
	}

	return &policy, nil
}
