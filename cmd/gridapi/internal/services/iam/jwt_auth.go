package iam

import (
	"context"
	"fmt"
	"strings"

	"github.com/xenitab/go-oidc-middleware/oidctoken"
	"github.com/xenitab/go-oidc-middleware/options"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// JWTAuthenticator authenticates requests using JWT bearer tokens.
//
// Implementation follows Phase 3 specification:
//  1. Extract "Authorization: Bearer <token>" header
//  2. Return (nil, nil) if not present
//  3. Verify JWT signature using existing auth.Verifier logic
//  4. Extract claims: sub, email, groups, jti
//  5. Check JTI revocation
//  6. Resolve user/service account (JIT provision if needed)
//  7. Call ResolveRoles() using immutable cache
//  8. Construct Principal with all fields populated
//  9. Return Principal
//
// This authenticator is stateless and thread-safe.
type JWTAuthenticator struct {
	cfg             *config.Config
	tokenHandler    *oidctoken.TokenHandler[map[string]any]
	users           repository.UserRepository
	serviceAccounts repository.ServiceAccountRepository
	revokedJTIs     repository.RevokedJTIRepository
	iamService      Service // Reference to parent IAM service for ResolveRoles
}

// NewJWTAuthenticator creates a new JWT authenticator.
//
// This constructor initializes the OIDC token handler using the same logic
// as auth.NewVerifier, but adapted for the Authenticator interface pattern.
func NewJWTAuthenticator(
	cfg *config.Config,
	users repository.UserRepository,
	serviceAccounts repository.ServiceAccountRepository,
	revokedJTIs repository.RevokedJTIRepository,
	iamService Service,
) (*JWTAuthenticator, error) {
	var issuer, clientID string
	var isInternalProvider bool

	// Mode detection (same as auth.NewVerifier)
	if cfg.OIDC.ExternalIdP != nil {
		// Mode 1: External IdP Only
		issuer = cfg.OIDC.ExternalIdP.Issuer
		clientID = cfg.OIDC.ExternalIdP.ClientID
		isInternalProvider = false
	} else if cfg.OIDC.Issuer != "" {
		// Mode 2: Internal IdP Only
		issuer = cfg.OIDC.Issuer
		clientID = cfg.OIDC.ClientID
		isInternalProvider = true
	} else {
		// Auth disabled - return nil authenticator
		return nil, nil
	}

	if issuer == "" {
		return nil, fmt.Errorf("oidc issuer is required")
	}
	if clientID == "" {
		return nil, fmt.Errorf("oidc client id is required")
	}

	oidcOpts := []options.Option{
		options.WithIssuer(issuer),
		options.WithRequiredAudience(clientID),
	}

	// CRITICAL: For internal provider, use lazy load to avoid race condition
	if isInternalProvider {
		oidcOpts = append(oidcOpts, options.WithLazyLoadJwks(true))
	}

	tokenHandler, err := oidctoken.New[map[string]any](nil, oidcOpts...)
	if err != nil {
		return nil, fmt.Errorf("initialize oidc token handler: %w", err)
	}

	return &JWTAuthenticator{
		cfg:             cfg,
		tokenHandler:    tokenHandler,
		users:           users,
		serviceAccounts: serviceAccounts,
		revokedJTIs:     revokedJTIs,
		iamService:      iamService,
	}, nil
}

// Authenticate extracts and validates JWT bearer tokens.
//
// Returns:
//   - (nil, nil) if no Authorization header present (no credentials for this authenticator)
//   - (nil, error) if authentication fails (invalid token, revoked JTI, etc.)
//   - (*Principal, nil) if authentication succeeds
func (a *JWTAuthenticator) Authenticate(ctx context.Context, req AuthRequest) (*Principal, error) {
	// Step 1: Extract "Authorization: Bearer <token>" header
	authHeader := req.Headers.Get("Authorization")
	if authHeader == "" {
		// No credentials for this authenticator, try next
		return nil, nil
	}

	// Parse bearer token using same logic as auth.NewVerifier
	tokenStrings := [][]options.TokenStringOption{
		{}, // Default: Authorization header
	}
	token, err := oidctoken.GetTokenString(req.Headers.Get, tokenStrings)
	if err != nil || token == "" {
		// No valid token found, try next authenticator
		return nil, nil
	}

	trimmedToken := strings.TrimSpace(token)

	// Step 3: Verify JWT signature
	claims, err := a.tokenHandler.ParseToken(ctx, trimmedToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Step 4: Extract claims
	jti, ok := claims["jti"].(string)
	if !ok || jti == "" {
		return nil, fmt.Errorf("token missing jti claim")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return nil, fmt.Errorf("token missing sub claim")
	}

	// Extract email (optional for service accounts)
	email, _ := claims["email"].(string)

	// Extract name (optional)
	name, _ := claims["name"].(string)

	// Extract groups (optional, may be empty for users with no groups)
	groups := a.extractGroups(claims)

	// Step 5: Check JTI revocation
	isRevoked, err := a.revokedJTIs.IsRevoked(ctx, jti)
	if err != nil {
		return nil, fmt.Errorf("check revocation status: %w", err)
	}
	if isRevoked {
		return nil, fmt.Errorf("token has been revoked")
	}

	// Step 6: Resolve user/service account (JIT provision if needed)
	user, serviceAccount, err := a.resolveIdentity(ctx, sub, email, name, groups)
	if err != nil {
		return nil, fmt.Errorf("resolve identity: %w", err)
	}

	var internalID string
	var principalID string
	var principalType PrincipalType

	if user != nil {
		internalID = user.ID
		principalID = fmt.Sprintf("user:%s", user.PrincipalSubject())
		principalType = PrincipalTypeUser
	} else if serviceAccount != nil {
		internalID = serviceAccount.ID
		principalID = fmt.Sprintf("service_account:%s", serviceAccount.Name)
		principalType = PrincipalTypeServiceAccount
	} else {
		return nil, fmt.Errorf("identity resolution failed")
	}

	// Step 7: Resolve roles using immutable cache
	roles, err := a.iamService.ResolveRoles(ctx, internalID, groups, principalType == PrincipalTypeUser)
	if err != nil {
		return nil, fmt.Errorf("resolve roles: %w", err)
	}

	// Step 8: Construct Principal
	principal := &Principal{
		Subject:     sub,
		PrincipalID: principalID,
		InternalID:  internalID,
		Email:       email,
		Name:        name,
		SessionID:   "", // No session for JWT auth
		Groups:      groups,
		Roles:       roles,
		Type:        principalType,
	}

	return principal, nil
}

// extractGroups extracts groups from JWT claims using configured claim field.
func (a *JWTAuthenticator) extractGroups(claims map[string]any) []string {
	// Use configured claim field from config
	claimField := a.cfg.OIDC.GroupsClaimField
	if claimField == "" {
		claimField = "groups" // Default
	}

	claimPath := a.cfg.OIDC.GroupsClaimPath

	// Extract groups using existing helper
	groups, err := auth.ExtractGroups(claims, claimField, claimPath)
	if err != nil {
		// Groups extraction failed, return empty (user may have no groups)
		return []string{}
	}

	return groups
}

// resolveIdentity resolves the user or service account from the JWT subject.
//
// Implementation:
//   - Look up user by subject
//   - If not found and email present, JIT provision user
//   - If email not present, look up service account
//   - If service account not found, JIT provision service account
//
// This matches the existing behavior in authn middleware.
func (a *JWTAuthenticator) resolveIdentity(
	ctx context.Context,
	sub, email, name string,
	groups []string,
) (*models.User, *models.ServiceAccount, error) {
	// Try to find existing user by subject
	user, err := a.users.GetBySubject(ctx, sub)
	if err == nil && user != nil {
		// User found, update last login
		_ = a.users.UpdateLastLogin(ctx, user.ID)
		return user, nil, nil
	}

	// User not found - check if this is a user (has email) or service account
	if email != "" {
		// JIT provision user
		subjectPtr := &sub
		user = &models.User{
			Subject:     subjectPtr,
			Email:       email,
			Name:        name,
			DisabledAt:  nil,
			LastLoginAt: nil,
		}
		if err := a.users.Create(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("create user: %w", err)
		}
		return user, nil, nil
	}

	// No email - this is a service account (client credentials flow)
	// Extract client_id from "sub" claim (strip "sa:" prefix if present)
	clientID := sub
	if extractedID, err := auth.ExtractServiceAccountID(sub); err == nil {
		clientID = extractedID
	}

	// Try to find existing service account by client_id
	serviceAccount, err := a.serviceAccounts.GetByClientID(ctx, clientID)
	if err == nil && serviceAccount != nil {
		// Service account found, update last used
		_ = a.serviceAccounts.UpdateLastUsed(ctx, serviceAccount.ID)
		return nil, serviceAccount, nil
	}

	// Service account not found - JIT provision for External IdP only
	if a.cfg.OIDC.ExternalIdP != nil {
		// Mode 1: External IdP (Keycloak) - JIT provision service accounts
		// These service accounts are managed by the External IdP, so we don't have their secrets
		// Use a special marker for ClientSecretHash to indicate external management
		serviceAccount = &models.ServiceAccount{
			ClientID:         clientID,               // Use extracted client_id (without prefix)
			ClientSecretHash: "EXTERNAL_IDP_MANAGED", // Marker: secret managed by external IdP
			Name:             clientID,               // Use client_id as name (can be updated by admin)
			Description:      "Auto-provisioned from External IdP",
			ScopeLabels:      make(models.LabelMap), // Empty scope labels
			CreatedBy:        auth.SystemUserID,     // System-provisioned
			Disabled:         false,
		}

		if err := a.serviceAccounts.Create(ctx, serviceAccount); err != nil {
			return nil, nil, fmt.Errorf("JIT provision service account: %w", err)
		}

		return nil, serviceAccount, nil
	}

	// Mode 2: Internal IdP - service accounts must be pre-created via bootstrap
	return nil, nil, fmt.Errorf("service account not found for client_id=%s (JIT provisioning only supported for External IdP)", clientID)
}
