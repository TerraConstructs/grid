package sdk

import "testing"

func TestLabelValidatorBasic(t *testing.T) {
	validator := NewLabelValidator(nil)
	labels := LabelMap{"env": "prod", "active": true}
	if err := validator.Validate(labels); err != nil {
		t.Fatalf("expected labels to be valid, got %v", err)
	}

	labels["Invalid-Key"] = "value"
	if err := validator.Validate(labels); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestLabelValidatorWithPolicy(t *testing.T) {
	policyJSON := `{"allowed_keys":{"env":{}},"allowed_values":{"env":["prod"]},"max_keys":2,"max_value_len":10}`
	policy, err := ParsePolicyDefinition(policyJSON)
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}

	validator := NewLabelValidator(policy)

	labels := LabelMap{"env": "prod"}
	if err := validator.Validate(labels); err != nil {
		t.Fatalf("expected valid labels, got %v", err)
	}

	labels["team"] = "core"
	if err := validator.Validate(labels); err == nil {
		t.Fatal("expected error for disallowed key")
	}
}
