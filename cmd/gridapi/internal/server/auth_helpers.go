package server

import (
	"context"
	"fmt"
	"log"

	"github.com/golang-jwt/jwt/v5"

	gridmiddleware "github.com/terraconstructs/grid/cmd/gridapi/internal/middleware"
)

// resolveEffectiveRoles aggregates roles from both direct user-role assignments and group-role mappings.
// For external IdP users, it decodes the ID token to extract group claims.
// For internal IdP users (no ID token), it returns only direct role assignments.
//
// Parameters:
//   - ctx: context for database operations
//   - deps: authentication dependencies with repository access
//   - userID: user's database ID
//   - sessionID: session ID (required to fetch ID token for external IdP users)
//   - isExternalIdP: whether this is an external IdP user (determined by presence of Subject field)
//
// Returns:
//   - roles: deduplicated list of role names
//   - groups: group names extracted from JWT (for external IdP users only)
//   - error: if any database operation fails
func resolveEffectiveRoles(
	ctx context.Context,
	deps *gridmiddleware.AuthnDependencies,
	userID string,
	sessionID string,
	isExternalIdP bool,
) ([]string, []string, error) {
	// Get direct role assignments for this user
	directRoles, err := getUserRoles(ctx, deps, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get direct user roles: %w", err)
	}

	// If internal IdP, return direct roles only
	if !isExternalIdP {
		return directRoles, nil, nil
	}

	// External IdP: also resolve group-based roles
	var groups []string
	var groupRoles []string

	// Try to get ID token from session
	idToken, err := getIDTokenFromSession(ctx, deps, sessionID)
	if err != nil {
		log.Printf("warning: could not retrieve ID token for session %s: %v", sessionID, err)
		// Continue with direct roles only
	} else if idToken != "" {
		// Extract groups from ID token
		groups, err = extractGroupsFromIDToken(idToken)
		if err != nil {
			log.Printf("warning: could not extract groups from ID token: %v", err)
		} else if len(groups) > 0 {
			// Resolve roles for each group
			groupRoles, err = getGroupRoles(ctx, deps, groups)
			if err != nil {
				log.Printf("warning: could not resolve group roles: %v", err)
			}
		}
	}

	// Merge and deduplicate roles
	allRoles := mergeAndDeduplicateRoles(directRoles, groupRoles)
	return allRoles, groups, nil
}

// getUserRoles fetches all direct role assignments for a user from the user_roles table.
func getUserRoles(ctx context.Context, deps *gridmiddleware.AuthnDependencies, userID string) ([]string, error) {
	if deps == nil || deps.UserRoles == nil {
		return nil, nil
	}

	userRoles, err := deps.UserRoles.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}

	roleNames := make([]string, 0, len(userRoles))
	for _, ur := range userRoles {
		role, err := deps.Roles.GetByID(ctx, ur.RoleID)
		if err != nil {
			log.Printf("warning: could not fetch role %s: %v", ur.RoleID, err)
			continue
		}
		roleNames = append(roleNames, role.Name)
	}

	return roleNames, nil
}

// getGroupRoles fetches role assignments for a list of groups from the group_roles table.
func getGroupRoles(ctx context.Context, deps *gridmiddleware.AuthnDependencies, groups []string) ([]string, error) {
	if deps == nil || deps.GroupRoles == nil || deps.Roles == nil {
		return nil, nil
	}

	roleNames := make([]string, 0)

	for _, group := range groups {
		groupRoles, err := deps.GroupRoles.GetByGroupName(ctx, group)
		if err != nil {
			log.Printf("warning: could not fetch group roles for %s: %v", group, err)
			continue
		}

		for _, gr := range groupRoles {
			role, err := deps.Roles.GetByID(ctx, gr.RoleID)
			if err != nil {
				log.Printf("warning: could not fetch role %s: %v", gr.RoleID, err)
				continue
			}
			roleNames = append(roleNames, role.Name)
		}
	}

	return roleNames, nil
}

// getIDTokenFromSession retrieves the ID token string from a session record.
// Returns empty string if session not found or has no ID token.
func getIDTokenFromSession(ctx context.Context, deps *gridmiddleware.AuthnDependencies, sessionID string) (string, error) {
	if deps == nil || deps.Sessions == nil || sessionID == "" {
		return "", nil
	}

	session, err := deps.Sessions.GetByID(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("get session by ID: %w", err)
	}

	return session.IDToken, nil
}

// extractGroupsFromIDToken decodes a JWT ID token and extracts the "groups" claim.
// The token is expected to be unsigned (we trust it from the session storage).
// Returns an empty slice if no groups claim is present.
func extractGroupsFromIDToken(idToken string) ([]string, error) {
	// Parse without verification (token is already validated and stored in DB)
	claims := jwt.MapClaims{}
	_, _, err := new(jwt.Parser).ParseUnverified(idToken, claims)
	if err != nil {
		return nil, fmt.Errorf("parse ID token: %w", err)
	}

	// Extract "groups" claim (may be string or []interface{})
	if groupsClaim, ok := claims["groups"]; ok {
		switch v := groupsClaim.(type) {
		case []interface{}:
			groups := make([]string, 0, len(v))
			for _, g := range v {
				if gs, ok := g.(string); ok {
					groups = append(groups, gs)
				}
			}
			return groups, nil
		case string:
			return []string{v}, nil
		}
	}

	return nil, nil
}

// mergeAndDeduplicateRoles combines two slices of role names and returns unique values.
func mergeAndDeduplicateRoles(direct, group []string) []string {
	if len(direct) == 0 && len(group) == 0 {
		return nil
	}

	// Use a map to track unique role names
	seen := make(map[string]bool)
	result := make([]string, 0, len(direct)+len(group))

	// Add direct roles
	for _, r := range direct {
		if !seen[r] {
			seen[r] = true
			result = append(result, r)
		}
	}

	// Add group roles
	for _, r := range group {
		if !seen[r] {
			seen[r] = true
			result = append(result, r)
		}
	}

	return result
}
