package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

// NewSessionInterceptor creates a Connect interceptor for session cookie authentication.
// This enables webapp requests (using session cookies) to authenticate with Connect RPC endpoints.
//
// Flow:
// 1. Extract grid.session cookie from request headers
// 2. Validate session in database
// 3. Load user/service account
// 4. Create synthetic JWT claims
// 5. Call ResolvePrincipal (shared with authn middleware)
// 6. Set principal in context
//
// Used by: Webapp (session cookie-based authentication)
// Complements: JWT interceptor (for CLI Bearer token authentication)
func NewSessionInterceptor(cfg *config.Config, deps AuthnDependencies) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// STEP 1: Extract session cookie from request headers
			cookieValue := extractCookieFromHeaders(req.Header(), auth.SessionCookieName)
			if cookieValue == "" {
				// No cookie - skip (let JWT interceptor or authz handle)
				return next(ctx, req)
			}

			// STEP 2: Hash cookie value for database lookup
			tokenHash := auth.HashToken(cookieValue)

			// STEP 3: Look up session in database
			session, err := deps.Sessions.GetByTokenHash(ctx, tokenHash)
			if err != nil {
				log.Printf("session lookup failed: %v", err)
				// Don't fail - let authz interceptor return 401
				return next(ctx, req)
			}
			if session == nil {
				// Invalid cookie - skip
				return next(ctx, req)
			}

			// STEP 4: Validate session not revoked
			if session.Revoked {
				// Session revoked - skip (authz will return 401)
				return next(ctx, req)
			}

			// STEP 5: Validate session not expired
			if time.Now().After(session.ExpiresAt) {
				// Session expired - skip (authz will return 401)
				return next(ctx, req)
			}

			// STEP 6: Determine subject from session (user or service account)
			var subject string
			if session.UserID != nil && *session.UserID != "" {
				// Load user to get subject
				user, err := deps.Users.GetByID(ctx, *session.UserID)
				if err != nil {
					log.Printf("failed to load user %s for session: %v", *session.UserID, err)
					return next(ctx, req)
				}

				// Check if user is disabled
				if user.DisabledAt != nil {
					// User disabled - skip (authz will return 401)
					return next(ctx, req)
				}

				subject = user.PrincipalSubject()
			} else if session.ServiceAccountID != nil && *session.ServiceAccountID != "" {
				// Load service account
				sa, err := deps.ServiceAccounts.GetByID(ctx, *session.ServiceAccountID)
				if err != nil {
					log.Printf("failed to load service account %s for session: %v", *session.ServiceAccountID, err)
					return next(ctx, req)
				}

				// Check if service account is disabled
				if sa.Disabled {
					// Service account disabled - skip (authz will return 401)
					return next(ctx, req)
				}

				subject = fmt.Sprintf("sa:%s", sa.ClientID)
			} else {
				log.Printf("session %s has no user_id or service_account_id", session.ID)
				return next(ctx, req)
			}

			// STEP 7: Create synthetic JWT claims (same pattern as session.go)
			syntheticClaims := map[string]interface{}{
				"sub": subject,
				"jti": session.ID, // Use session ID as JTI
			}

			// STEP 8: Call shared ResolvePrincipal function (from grid-3fbc)
			// This performs: identity resolution, role aggregation, Casbin grouping
			principal, groups, err := ResolvePrincipal(ctx, syntheticClaims, tokenHash, deps, cfg)
			if err != nil {
				log.Printf("failed to resolve principal for session %s: %v", session.ID, err)
				return next(ctx, req)
			}

			// STEP 9: Update last used timestamp (best effort, non-blocking)
			go func() {
				updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := deps.Sessions.UpdateLastUsed(updateCtx, session.ID); err != nil {
					log.Printf("warning: failed to update session last_used: %v", err)
				}
			}()

			// STEP 10: Set principal and groups in context
			ctx = auth.SetUserContext(ctx, *principal)
			ctx = auth.SetGroupsContext(ctx, groups)

			return next(ctx, req)
		})
	})
}

// extractCookieFromHeaders parses the Cookie header from a Connect request
// and extracts the value for the specified cookie name.
//
// Cookie header format: "name1=value1; name2=value2"
func extractCookieFromHeaders(headers http.Header, cookieName string) string {
	cookieHeader := headers.Get("Cookie")
	if cookieHeader == "" {
		return ""
	}

	// Parse cookies
	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		parts := strings.SplitN(strings.TrimSpace(cookie), "=", 2)
		if len(parts) == 2 && parts[0] == cookieName {
			return parts[1]
		}
	}

	return ""
}
