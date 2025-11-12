package middleware

import (
	"context"
	"fmt"
	"log"
	"strings"

	"connectrpc.com/connect"
	"github.com/xenitab/go-oidc-middleware/oidctoken"
	"github.com/xenitab/go-oidc-middleware/options"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

// NewJWTInterceptor creates a Connect interceptor for JWT bearer token authentication.
// This enables CLI tools (gridctl) and SDK clients to authenticate with Connect RPC endpoints.
//
// Flow:
// 1. Extract Authorization: Bearer header
// 2. Validate JWT signature and claims
// 3. Check JTI not revoked
// 4. Call ResolvePrincipal (shared with session interceptor)
// 5. Set principal in context
//
// Used by: CLI (gridctl), SDK clients, API clients with bearer tokens
// Complements: Session interceptor (for webapp cookie authentication)
func NewJWTInterceptor(cfg *config.Config, deps AuthnDependencies) (connect.UnaryInterceptorFunc, error) {
	// Determine OIDC configuration (same logic as NewVerifier in jwt.go)
	var issuer, clientID string
	var isInternalProvider bool

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
		// Auth disabled - return no-op interceptor
		return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return next
		}), nil
	}

	// Create OIDC token handler for JWT verification
	oidcOpts := []options.Option{
		options.WithIssuer(issuer),
		options.WithRequiredAudience(clientID),
	}

	// For internal provider, use lazy load (same as jwt.go)
	if isInternalProvider {
		oidcOpts = append(oidcOpts, options.WithLazyLoadJwks(true))
	}

	tokenHandler, err := oidctoken.New[map[string]any](nil, oidcOpts...)
	if err != nil {
		return nil, fmt.Errorf("initialise oidc token handler for JWT interceptor: %w", err)
	}

	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// STEP 1: Extract Authorization header
			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				// No auth header - skip (let session interceptor or authz handle)
				return next(ctx, req)
			}

			// STEP 2: Parse "Bearer <token>"
			if !strings.HasPrefix(authHeader, "Bearer ") {
				// Not a bearer token - skip
				return next(ctx, req)
			}
			tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if tokenString == "" {
				return next(ctx, req)
			}

			// STEP 3: Verify JWT signature and extract claims
			claims, err := tokenHandler.ParseToken(ctx, tokenString)
			if err != nil {
				log.Printf("JWT verification failed: %v", err)
				// Invalid JWT - skip (let authz return 401)
				return next(ctx, req)
			}

			// STEP 4: Validate JTI claim exists
			jti, ok := auth.JTIFromClaims(claims)
			if !ok {
				log.Printf("JWT missing jti claim")
				return next(ctx, req)
			}

			// STEP 5: Check JTI not revoked
			isRevoked, err := deps.RevokedJTIs.IsRevoked(ctx, jti)
			if err != nil {
				log.Printf("error checking jti revocation: %v", err)
				return next(ctx, req)
			}
			if isRevoked {
				// Token revoked - skip (let authz return 401)
				return next(ctx, req)
			}

			// STEP 6: Hash token for session lookup
			tokenHash := auth.HashToken(jti)

			// STEP 7: Call shared ResolvePrincipal function (from grid-3fbc)
			// This performs: identity resolution, role aggregation, Casbin grouping
			principal, groups, err := ResolvePrincipal(ctx, claims, tokenHash, deps, cfg)
			if err != nil {
				log.Printf("failed to resolve principal from JWT: %v", err)
				return next(ctx, req)
			}

			// STEP 8: Set principal and groups in context
			ctx = auth.SetUserContext(ctx, *principal)
			ctx = auth.SetGroupsContext(ctx, groups)

			return next(ctx, req)
		})
	}), nil
}
