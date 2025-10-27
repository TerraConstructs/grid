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
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
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

			// STEP 4: Extract subject and check identity disabled
			subject, _ := claims["sub"].(string)
			if subject == "" {
				http.Error(w, "invalid token: missing subject", http.StatusUnauthorized)
				return
			}

			// Check if identity is disabled
			principal, identityDisabled, err := resolvePrincipal(ctx, deps, cfg, claims, subject)
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
// In External IdP mode, automatically provisions users on first authentication (JIT provisioning).
// Returns the principal, whether the identity is disabled, and any error.
func resolvePrincipal(ctx context.Context, deps AuthnDependencies, cfg *config.Config, claims map[string]interface{}, subject string) (auth.AuthenticatedPrincipal, bool, error) {
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
		// Check if user not found and we're in External IdP mode (JIT provisioning)
		if err == sql.ErrNoRows || strings.Contains(err.Error(), "user not found") {
			// Only auto-provision in External IdP mode (Mode 1)
			if cfg.OIDC.IsInternalIdPMode() {
				// Internal IdP mode - users must be pre-created
				return auth.AuthenticatedPrincipal{}, false, fmt.Errorf("user not found: %s", subject)
			}
			log.Printf("User %s not found, attempting JIT provisioning from external IdP", subject)
			var jitErr error
			user, jitErr = createUserFromExternalJWT(ctx, deps, claims, cfg)
			if jitErr != nil {
				return auth.AuthenticatedPrincipal{}, false, fmt.Errorf("JIT provision user: %w", jitErr)
			}
			// Successfully provisioned, continue with the newly created user
		} else {
			// Other database errors
			return auth.AuthenticatedPrincipal{}, false, fmt.Errorf("get user: %w", err)
		}
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

// createUserFromExternalJWT creates a new user record from external IdP JWT claims (JIT provisioning).
// This is called when a user authenticates via external IdP but doesn't exist in Grid's database yet.
// Returns the created user and any error encountered.
func createUserFromExternalJWT(ctx context.Context, deps AuthnDependencies, claims map[string]interface{}, cfg *config.Config) (*models.User, error) {
	// Extract required subject claim
	subject, ok := claims["sub"].(string)
	if !ok || subject == "" {
		return nil, fmt.Errorf("missing or invalid sub claim")
	}

	// Extract email claim (configurable field)
	// For service account tokens, email may not be present - generate synthetic email
	emailClaimField := cfg.OIDC.EmailClaimField
	if emailClaimField == "" {
		emailClaimField = "email" // Default
	}
	email := ""
	if emailRaw, ok := claims[emailClaimField]; ok {
		if emailStr, ok := emailRaw.(string); ok && emailStr != "" {
			email = emailStr
		}
	}

	// If no email claim, generate synthetic email for service accounts
	// This handles Keycloak service account tokens which don't have email by default
	if email == "" {
		email = fmt.Sprintf("%s@external-idp.local", subject)
		log.Printf("No email claim found for subject %s, using synthetic email: %s", subject, email)
	}

	// Extract optional name claim
	name := ""
	if nameRaw, ok := claims["name"]; ok {
		if nameStr, ok := nameRaw.(string); ok {
			name = nameStr
		}
	}

	// Create user model
	newUser := &models.User{
		Subject: &subject,
		Email:   email,
		Name:    name,
	}

	// Attempt to create user
	if err := deps.Users.Create(ctx, newUser); err != nil {
		// Handle unique constraint violations (race conditions or email conflicts)
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			// First, try to find by subject (race condition - another request created the same user)
			user, lookupErr := deps.Users.GetBySubject(ctx, subject)
			if lookupErr == nil {
				log.Printf("JIT provisioning race condition resolved: user %s already exists", subject)
				return user, nil
			}

			// Subject lookup failed, check if email conflict (pre-existing user with same email)
			user, emailErr := deps.Users.GetByEmail(ctx, email)
			if emailErr == nil {
				// User with this email exists
				if user.Subject == nil || *user.Subject == "" {
					// Subject is empty - link this external IdP subject to existing user
					user.Subject = &subject
					if updateErr := deps.Users.Update(ctx, user); updateErr != nil {
						return nil, fmt.Errorf("link external subject to existing user: %w", updateErr)
					}
					log.Printf("JIT provisioning linked external IdP subject %s to existing user (email=%s)", subject, email)
					return user, nil
				} else if *user.Subject != subject {
					// Subject exists but differs - policy decision: reject
					return nil, fmt.Errorf("email %s already registered with different IdP subject (cannot re-link)", email)
				}
				// Subject matches - should not reach here, but return user anyway
				return user, nil
			}

			// Neither subject nor email found - return original error
			return nil, fmt.Errorf("create user (unique constraint, but user not found): %w", err)
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Create succeeded, but ID is not populated by repository.Create()
	// Re-read user by subject to get the generated ID
	user, err := deps.Users.GetBySubject(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("re-read created user: %w", err)
	}

	log.Printf("JIT provisioned user from external IdP: subject=%s email=%s name=%s id=%s", subject, email, name, user.ID)
	return user, nil
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
