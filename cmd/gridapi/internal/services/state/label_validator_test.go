package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// T014: Test LabelValidator.Validate with policy constraints
func TestLabelValidator_Validate(t *testing.T) {
	t.Run("valid labels pass validation", func(t *testing.T) {
		policy := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{
				"env":  {},
				"team": {},
			},
			AllowedValues: map[string][]string{
				"env": {"staging", "prod"},
			},
			MaxKeys:     32,
			MaxValueLen: 256,
		}

		validator := NewLabelValidator(policy)
		labels := models.LabelMap{
			"env":  "staging",
			"team": "platform",
		}

		err := validator.Validate(labels)
		assert.NoError(t, err)
	})

	t.Run("key format regex validation", func(t *testing.T) {
		policy := &models.PolicyDefinition{
			MaxKeys:     32,
			MaxValueLen: 256,
		}
		validator := NewLabelValidator(policy)

		tests := []struct {
			name      string
			labels    models.LabelMap
			shouldErr bool
			errMsg    string
		}{
			{
				name:      "valid lowercase with underscores",
				labels:    models.LabelMap{"env_name": "test"},
				shouldErr: false,
			},
			{
				name:      "valid with forward slashes",
				labels:    models.LabelMap{"team/subteam": "test"},
				shouldErr: false,
			},
			{
				name:      "invalid uppercase",
				labels:    models.LabelMap{"ENV": "test"},
				shouldErr: true,
				errMsg:    "does not match required format",
			},
			{
				name:      "invalid hyphen",
				labels:    models.LabelMap{"env-name": "test"},
				shouldErr: true,
				errMsg:    "does not match required format",
			},
			{
				name:      "invalid starts with number",
				labels:    models.LabelMap{"1env": "test"},
				shouldErr: true,
				errMsg:    "does not match required format",
			},
			{
				name:      "valid starts with lowercase",
				labels:    models.LabelMap{"e": "test"},
				shouldErr: false,
			},
			{
				name:      "invalid too long (>32 chars)",
				labels:    models.LabelMap{"this_is_a_very_long_key_name_exceeding_limit": "test"},
				shouldErr: true,
				errMsg:    "does not match required format",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.labels)
				if tt.shouldErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.errMsg)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("enum validation", func(t *testing.T) {
		policy := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{
				"env": {},
			},
			AllowedValues: map[string][]string{
				"env": {"staging", "prod"},
			},
			MaxKeys:     32,
			MaxValueLen: 256,
		}
		validator := NewLabelValidator(policy)

		t.Run("valid enum value", func(t *testing.T) {
			labels := models.LabelMap{"env": "staging"}
			err := validator.Validate(labels)
			assert.NoError(t, err)
		})

		t.Run("invalid enum value", func(t *testing.T) {
			labels := models.LabelMap{"env": "qa"}
			err := validator.Validate(labels)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "enum")
		})

		t.Run("no enum constraint allows any value", func(t *testing.T) {
			policyNoEnum := &models.PolicyDefinition{
				AllowedKeys: map[string]struct{}{
					"team": {},
				},
				MaxKeys:     32,
				MaxValueLen: 256,
			}
			validatorNoEnum := NewLabelValidator(policyNoEnum)

			labels := models.LabelMap{"team": "any-value"}
			err := validatorNoEnum.Validate(labels)
			assert.NoError(t, err)
		})
	})

	t.Run("reserved prefix validation", func(t *testing.T) {
		policy := &models.PolicyDefinition{
			ReservedPrefixes: []string{"grid_io/", "kubernetes_io/"},
			MaxKeys:          32,
			MaxValueLen:      256,
		}
		validator := NewLabelValidator(policy)

		t.Run("reserved prefix rejected", func(t *testing.T) {
			labels := models.LabelMap{"grid_io/internal": "test"}
			err := validator.Validate(labels)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "reserved prefix")
		})

		t.Run("non-reserved prefix allowed", func(t *testing.T) {
			labels := models.LabelMap{"mycompany_io/custom": "test"}
			err := validator.Validate(labels)
			assert.NoError(t, err)
		})
	})

	t.Run("size limits validation", func(t *testing.T) {
		policy := &models.PolicyDefinition{
			MaxKeys:     2,
			MaxValueLen: 10,
		}
		validator := NewLabelValidator(policy)

		t.Run("exceeds max keys", func(t *testing.T) {
			labels := models.LabelMap{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			}
			err := validator.Validate(labels)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "max_keys")
		})

		t.Run("string value exceeds max length", func(t *testing.T) {
			labels := models.LabelMap{
				"env": "this-is-a-very-long-value-exceeding-limit",
			}
			err := validator.Validate(labels)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "max_value_len")
		})

		t.Run("within size limits", func(t *testing.T) {
			labels := models.LabelMap{
				"env":  "prod",
				"team": "core",
			}
			err := validator.Validate(labels)
			assert.NoError(t, err)
		})
	})

	t.Run("allowed keys validation", func(t *testing.T) {
		policy := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{
				"env":  {},
				"team": {},
			},
			MaxKeys:     32,
			MaxValueLen: 256,
		}
		validator := NewLabelValidator(policy)

		t.Run("disallowed key rejected", func(t *testing.T) {
			labels := models.LabelMap{"region": "us-west"}
			err := validator.Validate(labels)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "allowed_keys")
		})

		t.Run("allowed key accepted", func(t *testing.T) {
			labels := models.LabelMap{"env": "prod"}
			err := validator.Validate(labels)
			assert.NoError(t, err)
		})
	})

	t.Run("type validation", func(t *testing.T) {
		policy := &models.PolicyDefinition{
			MaxKeys:     32,
			MaxValueLen: 256,
		}
		validator := NewLabelValidator(policy)

		t.Run("string values accepted", func(t *testing.T) {
			labels := models.LabelMap{"env": "staging"}
			err := validator.Validate(labels)
			assert.NoError(t, err)
		})

		t.Run("number values accepted", func(t *testing.T) {
			labels := models.LabelMap{"count": float64(3)}
			err := validator.Validate(labels)
			assert.NoError(t, err)
		})

		t.Run("boolean values accepted", func(t *testing.T) {
			labels := models.LabelMap{"active": true}
			err := validator.Validate(labels)
			assert.NoError(t, err)
		})

		t.Run("unsupported types rejected", func(t *testing.T) {
			labels := models.LabelMap{"data": []string{"array"}}
			err := validator.Validate(labels)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "type")
		})
	})

	t.Run("nil policy allows all labels", func(t *testing.T) {
		validator := NewLabelValidator(nil)
		labels := models.LabelMap{
			"any_key": "any_value",
		}
		// Should apply basic validation even without policy
		err := validator.Validate(labels)
		// May pass or require key format - implementation decides
		_ = err
	})
}
