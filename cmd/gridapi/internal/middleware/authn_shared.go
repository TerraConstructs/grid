package middleware

import (
	"context"
	"fmt"
	"log"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

// ResolvePrincipal converts validated JWT claims into a fully-permissioned principal.
//
// This function performs the complete principal resolution pipeline:
// 1. Resolves identity (user or service account) from subject claim
// 2. JIT user provisioning for external IdP users (if needed)
// 3. Extracts groups from claims
// 4. Builds group→role mappings
// 5. Applies dynamic Casbin grouping policies
// 6. Resolves effective roles (user_roles ∪ group_roles)
// 7. Constructs AuthenticatedPrincipal with all permissions
//
// Used by:
// - Chi authn middleware (HTTP layer)
// - Session cookie interceptor (Connect RPC)
// - JWT bearer token interceptor (Connect RPC)
//
// Parameters:
//   - ctx: Request context for tracing/logging
//   - claims: Validated JWT claims (from JWT token or synthetic claims from session)
//   - tokenHash: Hash of the token/cookie for session lookup (optional, may be "")
//   - deps: All repository and enforcer dependencies
//   - cfg: Configuration for IdP mode and claim field mapping
//
// Returns:
//   - *auth.AuthenticatedPrincipal: Fully-resolved principal with roles and permissions
//   - []string: Groups extracted from claims (for context storage by caller)
//   - error: Any error during resolution (identity disabled, DB errors, etc.)
func ResolvePrincipal(
	ctx context.Context,
	claims map[string]interface{},
	tokenHash string,
	deps AuthnDependencies,
	cfg *config.Config,
) (*auth.AuthenticatedPrincipal, []string, error) {
	// STEP 1: Extract subject claim
	subject, ok := claims["sub"].(string)
	if !ok || subject == "" {
		return nil, nil, fmt.Errorf("invalid token: missing subject claim")
	}

	// STEP 2: Resolve principal identity (user or service account)
	// Calls existing resolvePrincipal helper from authn.go
	principal, identityDisabled, err := resolvePrincipal(ctx, deps, cfg, claims, subject, tokenHash)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve principal for subject %s: %w", subject, err)
	}
	if identityDisabled {
		return nil, nil, fmt.Errorf("account disabled: %s", subject)
	}

	// STEP 3: Extract groups from claims
	// Calls existing extractGroupsFromClaims helper from authn.go
	groups := extractGroupsFromClaims(claims, cfg)

	// STEP 4: Build group→role mappings
	// Calls existing buildGroupRoleMap helper from authn.go
	groupRoleMap, err := buildGroupRoleMap(ctx, deps, groups)
	if err != nil {
		return nil, nil, fmt.Errorf("build group→role map for subject %s: %w", subject, err)
	}

	// STEP 5: Apply dynamic Casbin grouping policies
	if err := auth.ApplyDynamicGroupings(deps.Enforcer, principal.PrincipalID, groups, groupRoleMap); err != nil {
		return nil, nil, fmt.Errorf("apply dynamic groupings for subject %s: %w", subject, err)
	}

	// STEP 6: Resolve effective roles (user_roles ∪ group_roles)
	roles, err := auth.GetEffectiveRoles(deps.Enforcer, principal.PrincipalID)
	if err != nil {
		// Log warning but don't fail - principal may have no roles yet
		log.Printf("warning: failed to get effective roles for %s: %v", principal.PrincipalID, err)
		principal.Roles = []string{}
	} else {
		// Trim Casbin role prefix when presenting externally
		// Calls existing stripRolePrefix helper from authn.go
		principal.Roles = stripRolePrefix(roles)
	}

	// Return principal and groups (groups stored in context by caller)
	return &principal, groups, nil
}