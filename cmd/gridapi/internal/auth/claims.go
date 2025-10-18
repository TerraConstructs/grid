package auth

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
)

// ExtractGroups handles both flat and nested group claims from JWT tokens
// Supports:
//   - Flat arrays: ["dev-team", "contractors"]
//   - Nested objects: [{"name": "dev-team", "type": "team"}] with claimPath="name"
//
// Reference: research.md §9 (lines 629-787), CLARIFICATIONS.md §1 (JWT claim config)
func ExtractGroups(claims map[string]interface{}, claimField string, claimPath string) ([]string, error) {
	rawValue, ok := claims[claimField]
	if !ok {
		// Groups claim not present - return empty list (not an error, user may have no groups)
		return []string{}, nil
	}

	// Try flat string array first: ["dev-team", "contractors"]
	if groups, ok := rawValue.([]interface{}); ok {
		result := make([]string, 0, len(groups))
		for _, g := range groups {
			if str, ok := g.(string); ok {
				result = append(result, str)
			}
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	// Try nested extraction if path provided: [{"name": "dev-team"}]
	if claimPath != "" {
		return extractNestedGroups(rawValue, claimPath)
	}

	return nil, fmt.Errorf("groups claim invalid format (expected []string or []object with path)")
}

// extractNestedGroups uses mapstructure to extract from nested objects
// Supports simple single-level paths like "name", "value", "id"
func extractNestedGroups(rawValue interface{}, path string) ([]string, error) {
	// For simple nested objects: [{"name": "dev-team"}] with path="name"
	// Only support single-level paths (YAGNI for complex nested structures)
	if path == "name" || path == "value" || path == "id" {
		var objects []map[string]interface{}
		if err := mapstructure.Decode(rawValue, &objects); err != nil {
			return nil, fmt.Errorf("failed to decode nested groups: %w", err)
		}

		result := make([]string, 0, len(objects))
		for _, obj := range objects {
			if val, ok := obj[path].(string); ok {
				result = append(result, val)
			}
		}
		return result, nil
	}

	// For complex paths (future): consider gjson if demand arises
	return nil, fmt.Errorf("complex nested paths not yet supported (path: %s)", path)
}

// ExtractClaimString extracts a string claim from JWT claims
// Generic helper for extracting string values from configurable claim fields
func ExtractClaimString(claims map[string]interface{}, claimField string) (string, error) {
	rawValue, ok := claims[claimField]
	if !ok {
		return "", fmt.Errorf("claim field %s not found", claimField)
	}

	value, ok := rawValue.(string)
	if !ok {
		return "", fmt.Errorf("claim field %s is not a string", claimField)
	}

	if value == "" {
		return "", fmt.Errorf("claim field %s is empty", claimField)
	}

	return value, nil
}

// ExtractSubjectFromClaims extracts the user subject ID from JWT claims
// Uses configurable claim field (default: "sub")
func ExtractSubjectFromClaims(claims map[string]interface{}, claimField string) (string, error) {
	return ExtractClaimString(claims, claimField)
}

// ExtractEmailFromClaims extracts the email from JWT claims
// Uses configurable claim field (default: "email")
func ExtractEmailFromClaims(claims map[string]interface{}, claimField string) (string, error) {
	return ExtractClaimString(claims, claimField)
}

// ExtractNameFromClaims extracts the name from JWT claims (optional)
// Uses standard "name" claim field
func ExtractNameFromClaims(claims map[string]interface{}) string {
	rawValue, ok := claims["name"]
	if !ok {
		return ""
	}

	name, ok := rawValue.(string)
	if !ok {
		return ""
	}

	return name
}
