package iam

import (
	"fmt"
	"log"

	"github.com/casbin/casbin/v2"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
)

// AuthorizeWithRoles checks if ANY of the principal's roles grants permission for the requested action.
//
// This is a READ-ONLY authorization check that never mutates Casbin state. It queries the enforcer
// with each role in the principal's role list and returns true if at least one role allows the action.
//
// The enforcer is queried with role principals (e.g., "role:product-engineer") which are defined
// in the static Casbin policy. This eliminates the need for dynamic user→group→role mappings.
//
// Parameters:
//   - enforcer: Casbin enforcer loaded with static role-based policies
//   - roles: List of role names (without "role:" prefix) assigned to the principal
//   - obj: Object type (e.g., "state", "admin", "policy") or specific resource ID
//   - act: Action being requested (e.g., "state:create", "state:read", "admin:role:manage")
//   - labels: Resource-specific attributes for label-based filtering (e.g., state labels)
//
// Returns:
//   - bool: true if at least one role grants permission, false otherwise
//   - error: Any error during enforcement (e.g., enforcer failure, invalid policy)
//
// Thread Safety:
//   - This function is thread-safe and performs no state mutation
//   - Multiple goroutines can call this concurrently without locks
//
// Performance:
//   - O(n) where n = number of roles (typically 1-5 roles per user)
//   - Each enforcer.Enforce() call is fast (cached policy evaluation)
//   - No database queries or blocking I/O
//
// Example:
//
//	roles := []string{"product-engineer", "viewer"}
//	labels := map[string]interface{}{"env": "dev"}
//	allowed, err := AuthorizeWithRoles(enforcer, roles, "state", "state:read", labels)
//	if err != nil {
//	    return fmt.Errorf("authorization error: %w", err)
//	}
//	if !allowed {
//	    return fmt.Errorf("permission denied")
//	}
func AuthorizeWithRoles(
	enforcer casbin.IEnforcer,
	roles []string,
	obj, act string,
	labels map[string]interface{},
) (bool, error) {
	if enforcer == nil {
		return false, fmt.Errorf("casbin enforcer not initialized")
	}

	// Handle empty roles: deny by default (no roles = no permissions)
	if len(roles) == 0 {
		log.Printf("authorization denied: principal has no roles (obj=%s, act=%s)", obj, act)
		return false, nil
	}

	// Ensure labels is never nil (Casbin expects map[string]interface{})
	if labels == nil {
		labels = make(map[string]interface{})
	}

	// Try each role until one grants permission
	for _, roleName := range roles {
		// Convert role name to Casbin principal ID (e.g., "product-engineer" → "role:product-engineer")
		rolePrincipal := auth.RoleID(roleName)

		log.Printf("authorization check: role=%s, obj=%s, act=%s, labels=%v", rolePrincipal, obj, act, labels)

		// Query Casbin enforcer (READ-ONLY - no AddGroupingPolicy!)
		allowed, err := enforcer.Enforce(rolePrincipal, obj, act, labels)
		if err != nil {
			// Log error but continue checking other roles
			log.Printf("error checking role %s: %v", rolePrincipal, err)
			return false, fmt.Errorf("casbin enforce error for role %s: %w", rolePrincipal, err)
		}

		if allowed {
			log.Printf("authorization granted: role %s allows %s on %s", rolePrincipal, act, obj)
			return true, nil // At least one role allows - grant permission
		}
	}

	// No role granted permission
	log.Printf("authorization denied: no role in %v allows %s on %s (labels=%v)", roles, act, obj, labels)
	return false, nil
}
