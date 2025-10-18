package middleware

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// AuthnDependencies bundles collaborators required by the authentication middleware.
type AuthnDependencies struct {
	Sessions        repository.SessionRepository // Required by other consumers (auth handlers)
	Users           repository.UserRepository
	UserRoles       repository.UserRoleRepository // Required by other consumers (auth handlers)
	ServiceAccounts repository.ServiceAccountRepository
	RevokedJTIs     repository.RevokedJTIRepository
	GroupRoles      repository.GroupRoleRepository
	Roles           repository.RoleRepository
	Enforcer        casbin.IEnforcer // Required for dynamic grouping setup
}

// NewAuthnMiddleware composes the Terraform shim, OIDC verifier, and JWT-based authentication
// with JTI revocation checking and dynamic Casbin grouping.
func NewAuthnMiddleware(cfg *config.Config, deps AuthnDependencies, verifierOpts ...auth.VerifierOption) (func(http.Handler) http.Handler, error) {
	if deps.RevokedJTIs == nil {
		return nil, errors.New("authn middleware requires revoked JTI repository")
	}
	if deps.Enforcer == nil {
		return nil, errors.New("authn middleware requires casbin enforcer")
	}

	verifier, err := auth.NewVerifier(cfg, verifierOpts...)
	if err != nil {
		return nil, fmt.Errorf("initialise oidc verifier: %w", err)
	}

	return func(next http.Handler) http.Handler {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// STEP 1: Get validated claims (set by jwt.go middleware)
			claims, hasClaims := auth.ClaimsFromContext(ctx)
			if !hasClaims {
				// No claims = public route (skipped by verifier)
				log.Printf("no claims found in context for %s %s, treating as public route", r.Method, r.URL.Path)
				next.ServeHTTP(w, r)
				return
			}

			// STEP 2: Extract JTI from claims
			jti, ok := auth.JTIFromClaims(claims)
			if !ok {
				log.Printf("missing jti claim in token for %s %s", r.Method, r.URL.Path)
				// Should never happen (jwt.go validates this)
				http.Error(w, "invalid token: missing jti", http.StatusUnauthorized)
				return
			}
			log.Printf("extracted jti from token for %s %s, testing for revocation", r.Method, r.URL.Path)

			// STEP 3: Check JTI revocation via repository
			isRevoked, err := deps.RevokedJTIs.IsRevoked(ctx, jti)
			if err != nil {
				// Log error but don't expose details to client
				log.Printf("error checking jti revocation for %s %s: %v", r.Method, r.URL.Path, err)
				http.Error(w, "authentication error", http.StatusInternalServerError)
				return
			}
			if isRevoked {
				http.Error(w, "token has been revoked", http.StatusUnauthorized)
				return
			}
			log.Printf("jti is valid for %s %s", r.Method, r.URL.Path)

			// STEP 4: Extract subject and check identity disabled
			subject, _ := claims["sub"].(string)
			if subject == "" {
				http.Error(w, "invalid token: missing subject", http.StatusUnauthorized)
				return
			}

			// Check if identity is disabled
			log.Printf("resolving principal for subject %s for %s %s", subject, r.Method, r.URL.Path)
			principal, identityDisabled, err := resolvePrincipal(ctx, deps, subject)
			if err != nil {
				log.Printf("error resolving principal for subject %s for %s %s: %v", subject, r.Method, r.URL.Path, err)
				http.Error(w, "authentication error", http.StatusInternalServerError)
				return
			}
			if identityDisabled {
				http.Error(w, "account disabled", http.StatusUnauthorized)
				return
			}

			// STEP 5: Extract groups and apply dynamic Casbin grouping
			groups := extractGroupsFromClaims(claims, cfg)
			groupRoleMap, err := buildGroupRoleMap(ctx, deps, groups)
			if err != nil {
				http.Error(w, "authorization setup failed", http.StatusInternalServerError)
				return
			}

			if err := auth.ApplyDynamicGroupings(deps.Enforcer, principal.PrincipalID, groups, groupRoleMap); err != nil {
				http.Error(w, "authorization setup failed", http.StatusInternalServerError)
				return
			}

			// Resolve effective roles for the principal
			if roles, err := auth.GetEffectiveRoles(deps.Enforcer, principal.PrincipalID); err == nil {
				// Trim Casbin role prefix when presenting externally
				principal.Roles = stripRolePrefix(roles)
			}

			// STEP 6: Store principal and groups in context for authz middleware
			ctx = auth.SetGroupsContext(ctx, groups)
			ctx = auth.SetUserContext(ctx, principal)

			next.ServeHTTP(w, r.WithContext(ctx))
		})

		return auth.TerraformBasicAuthShim(verifier(handler))
	}, nil
}

// resolvePrincipal resolves the authenticated principal from the subject claim.
// Returns the principal, whether the identity is disabled, and any error.
func resolvePrincipal(ctx context.Context, deps AuthnDependencies, subject string) (auth.AuthenticatedPrincipal, bool, error) {
	// Check if service account (subject starts with "sa:")
	if strings.HasPrefix(subject, "sa:") {
		clientID := strings.TrimPrefix(subject, "sa:")
		sa, err := deps.ServiceAccounts.GetByClientID(ctx, clientID)
		if err != nil {
			return auth.AuthenticatedPrincipal{}, false, fmt.Errorf("get service account: %w", err)
		}

		principal := auth.AuthenticatedPrincipal{
			Subject:     sa.ClientID,
			PrincipalID: auth.ServiceAccountID(sa.ClientID),
			InternalID:  sa.ID,
			Name:        sa.Name,
			Type:        auth.PrincipalTypeServiceAccount,
		}
		return principal, sa.Disabled, nil
	}

	// Otherwise, treat as user
	user, err := deps.Users.GetBySubject(ctx, subject)
	if err != nil {
		if err == sql.ErrNoRows {
			return auth.AuthenticatedPrincipal{}, false, fmt.Errorf("user not found: %s", subject)
		}
		return auth.AuthenticatedPrincipal{}, false, fmt.Errorf("get user: %w", err)
	}

	subjectID := user.PrincipalSubject()
	principal := auth.AuthenticatedPrincipal{
		Subject:     subjectID,
		PrincipalID: auth.UserID(subjectID),
		InternalID:  user.ID,
		Email:       user.Email,
		Name:        user.Name,
		Type:        auth.PrincipalTypeUser,
	}
	return principal, user.DisabledAt != nil, nil
}

// extractGroupsFromClaims extracts groups from JWT claims using configured claim field/path.
func extractGroupsFromClaims(claims map[string]interface{}, cfg *config.Config) []string {
	groups, _ := auth.ExtractGroups(claims, cfg.OIDC.GroupsClaimField, cfg.OIDC.GroupsClaimPath)
	return groups
}

// buildGroupRoleMap builds the map of group names to role names for dynamic grouping.
func buildGroupRoleMap(ctx context.Context, deps AuthnDependencies, groups []string) (map[string][]string, error) {
	groupRoleMap := make(map[string][]string, len(groups))
	roleNameCache := make(map[string]string)

	if deps.GroupRoles == nil || deps.Roles == nil {
		return groupRoleMap, nil
	}

	for _, group := range groups {
		assignments, err := deps.GroupRoles.GetByGroupName(ctx, group)
		if err != nil {
			return nil, fmt.Errorf("get group roles for %s: %w", group, err)
		}

		for _, assignment := range assignments {
			roleName, ok := roleNameCache[assignment.RoleID]
			if !ok {
				role, roleErr := deps.Roles.GetByID(ctx, assignment.RoleID)
				if roleErr != nil {
					return nil, fmt.Errorf("get role %s: %w", assignment.RoleID, roleErr)
				}
				roleName = role.Name
				roleNameCache[assignment.RoleID] = roleName
			}
			groupRoleMap[group] = append(groupRoleMap[group], roleName)
		}
	}

	return groupRoleMap, nil
}

func stripRolePrefix(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(roles))
	for _, role := range roles {
		trimmed = append(trimmed, strings.TrimPrefix(role, auth.PrefixRole))
	}
	return trimmed
}

// unauthenticated is a helper to return an unauthenticated error response.
func unauthenticated(w http.ResponseWriter) {
	http.Error(w, "unauthenticated", http.StatusUnauthorized)
}
