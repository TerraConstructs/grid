package auth

import (
	"fmt"

	"github.com/casbin/casbin/v2"
)

// ApplyDynamicGroupings mutates the enforcer with the transitive user→group→role chain
// derived from JWT groups and repository-backed group role mappings.
// Dynamic groupings are idempotent and cleared/re-applied on every authentication event.
//
// Note: Casbin's g() function correctly traverses the user→group→role hierarchy.
// We add user→group groupings and group→role groupings, and Casbin resolves the transitive
// permissions through its matcher evaluation.
func ApplyDynamicGroupings(
	enforcer casbin.IEnforcer,
	userPrincipal string,
	groups []string,
	groupRoles map[string][]string,
) error {
	if err := clearUserGroupings(enforcer, userPrincipal); err != nil {
		return fmt.Errorf("clear user groupings: %w", err)
	}

	// Add user→group groupings (for group membership tracking)
	for _, groupName := range groups {
		groupPrincipal := GroupID(groupName)
		if _, err := enforcer.AddGroupingPolicy(userPrincipal, groupPrincipal); err != nil {
			return fmt.Errorf("add user-group grouping: %w", err)
		}
	}

	// Add group→role groupings (for permission resolution)
	for groupName, roles := range groupRoles {
		groupPrincipal := GroupID(groupName)
		for _, roleName := range roles {
			rolePrincipal := RoleID(roleName)
			if _, err := enforcer.AddGroupingPolicy(groupPrincipal, rolePrincipal); err != nil {
				return fmt.Errorf("add group-role grouping: %w", err)
			}
		}
	}

	return nil
}

func clearUserGroupings(enforcer casbin.IEnforcer, userPrincipal string) error {
	roles, err := enforcer.GetRolesForUser(userPrincipal)
	if err != nil {
		return fmt.Errorf("get user roles: %w", err)
	}

	for _, role := range roles {
		if GetPrincipalType(role) == "group" {
			if _, err := enforcer.DeleteRoleForUser(userPrincipal, role); err != nil {
				return fmt.Errorf("delete user grouping for %s: %w", role, err)
			}
		}
	}
	return nil
}

// GetEffectiveRoles returns all effective roles for a user (via groups and direct assignments).
func GetEffectiveRoles(enforcer casbin.IEnforcer, userPrincipal string) ([]string, error) {
	immediateRoles, err := enforcer.GetRolesForUser(userPrincipal)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}

	effectiveRoles := make([]string, 0)
	for _, role := range immediateRoles {
		switch GetPrincipalType(role) {
		case "role":
			effectiveRoles = append(effectiveRoles, role)
		case "group":
			groupRoles, err := enforcer.GetRolesForUser(role)
			if err != nil {
				return nil, fmt.Errorf("get group roles: %w", err)
			}
			effectiveRoles = append(effectiveRoles, groupRoles...)
		}
	}
	return effectiveRoles, nil
}

// ResolveGroupRoles returns the roles currently attached to the given group.
func ResolveGroupRoles(enforcer casbin.IEnforcer, groupName string) ([]string, error) {
	groupPrincipal := GroupID(groupName)
	roles, err := enforcer.GetRolesForUser(groupPrincipal)
	if err != nil {
		return nil, fmt.Errorf("get group roles: %w", err)
	}
	return roles, nil
}
