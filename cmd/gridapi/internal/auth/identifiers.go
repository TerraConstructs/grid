package auth

import (
	"fmt"
	"strings"
)

// Prefix constants for Casbin identifiers
// These ensure consistent naming across user-role groupings, group-role mappings, and policy definitions
//
// Reference: PREFIX-CONVENTIONS.md §2-3, §9 (helper function patterns)
const (
	PrefixUser           = "user:"
	PrefixGroup          = "group:"
	PrefixServiceAccount = "sa:"
	PrefixRole           = "role:"
)

// UserID creates a Casbin user identifier with the standard prefix
// Example: UserID("alice@example.com") → "user:alice@example.com"
func UserID(id string) string {
	return PrefixUser + id
}

// GroupID creates a Casbin group identifier with the standard prefix
// Example: GroupID("dev-team") → "group:dev-team"
func GroupID(name string) string {
	return PrefixGroup + name
}

// ServiceAccountID creates a Casbin service account identifier with the standard prefix
// Example: ServiceAccountID("550e8400-e29b-41d4-a716-446655440000") → "sa:550e8400-e29b-41d4-a716-446655440000"
func ServiceAccountID(id string) string {
	return PrefixServiceAccount + id
}

// RoleID creates a Casbin role identifier with the standard prefix
// Example: RoleID("product-engineer") → "role:product-engineer"
func RoleID(name string) string {
	return PrefixRole + name
}

// ExtractUserID extracts the user ID from a Casbin principal identifier
// Returns the ID without prefix, or error if prefix mismatch
// Example: ExtractUserID("user:alice@example.com") → "alice@example.com", nil
func ExtractUserID(principal string) (string, error) {
	if !strings.HasPrefix(principal, PrefixUser) {
		return "", fmt.Errorf("invalid user principal: %s (expected prefix %s)", principal, PrefixUser)
	}
	return strings.TrimPrefix(principal, PrefixUser), nil
}

// ExtractGroupID extracts the group name from a Casbin principal identifier
// Returns the name without prefix, or error if prefix mismatch
// Example: ExtractGroupID("group:dev-team") → "dev-team", nil
func ExtractGroupID(principal string) (string, error) {
	if !strings.HasPrefix(principal, PrefixGroup) {
		return "", fmt.Errorf("invalid group principal: %s (expected prefix %s)", principal, PrefixGroup)
	}
	return strings.TrimPrefix(principal, PrefixGroup), nil
}

// ExtractServiceAccountID extracts the service account ID from a Casbin principal identifier
// Returns the ID without prefix, or error if prefix mismatch
// Example: ExtractServiceAccountID("sa:550e8400-e29b-41d4-a716-446655440000") → "550e8400-e29b-41d4-a716-446655440000", nil
func ExtractServiceAccountID(principal string) (string, error) {
	if !strings.HasPrefix(principal, PrefixServiceAccount) {
		return "", fmt.Errorf("invalid service account principal: %s (expected prefix %s)", principal, PrefixServiceAccount)
	}
	return strings.TrimPrefix(principal, PrefixServiceAccount), nil
}

// ExtractRoleID extracts the role name from a Casbin principal identifier
// Returns the name without prefix, or error if prefix mismatch
// Example: ExtractRoleID("role:product-engineer") → "product-engineer", nil
func ExtractRoleID(principal string) (string, error) {
	if !strings.HasPrefix(principal, PrefixRole) {
		return "", fmt.Errorf("invalid role principal: %s (expected prefix %s)", principal, PrefixRole)
	}
	return strings.TrimPrefix(principal, PrefixRole), nil
}

// GetPrincipalType returns the type of a Casbin principal (user, group, sa, role)
// Returns empty string if prefix not recognized
func GetPrincipalType(principal string) string {
	switch {
	case strings.HasPrefix(principal, PrefixUser):
		return "user"
	case strings.HasPrefix(principal, PrefixGroup):
		return "group"
	case strings.HasPrefix(principal, PrefixServiceAccount):
		return "service_account"
	case strings.HasPrefix(principal, PrefixRole):
		return "role"
	default:
		return ""
	}
}
