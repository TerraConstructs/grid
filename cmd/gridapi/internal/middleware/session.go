package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
)

// sessionContextKey is used to store session metadata in context
type sessionContextKey struct{}

var defaultSessionContextKey = sessionContextKey{}

// SessionMetadata contains session information extracted from cookie
type SessionMetadata struct {
	SessionID string
	TokenHash string
	ExpiresAt time.Time
}

// NewSessionAuthMiddleware creates middleware that handles cookie-based session authentication.
// It should be inserted BEFORE the JWT verifier in the middleware chain.
// This middleware extracts session cookies and creates synthetic JWT claims for the authn middleware.
//
// Flow:
// 1. Check for grid.session cookie
// 2. If found: validate session, create synthetic claims, set in context
// 3. If not found or invalid: pass through (JWT verifier will handle Bearer tokens)
//
// The downstream authn middleware will then process the claims (from cookie or JWT).
func NewSessionAuthMiddleware(deps AuthnDependencies) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Skip for public paths (same skipper as JWT)
			if shouldSkipSessionAuth(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Try to extract session cookie
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err != nil || cookie.Value == "" {
				// No cookie - continue to JWT verifier
				next.ServeHTTP(w, r)
				return
			}

			// Hash cookie value for database lookup
			tokenHash := auth.HashToken(cookie.Value)

			// Look up session in database
			session, err := deps.Sessions.GetByTokenHash(ctx, tokenHash)
			if err != nil {
				log.Printf("session lookup failed: %v", err)
				// Don't fail - maybe JWT will work
				next.ServeHTTP(w, r)
				return
			}
			if session == nil {
				// Invalid cookie - continue to JWT verifier
				next.ServeHTTP(w, r)
				return
			}

			// Validate session not revoked
			if session.Revoked {
				http.Error(w, "Session revoked", http.StatusUnauthorized)
				return
			}

			// Validate session not expired
			if time.Now().After(session.ExpiresAt) {
				http.Error(w, "Session expired", http.StatusUnauthorized)
				return
			}

			// Determine subject from session
			var subject string
			if session.UserID != nil && *session.UserID != "" {
				// Load user to get subject
				user, err := deps.Users.GetByID(ctx, *session.UserID)
				if err != nil {
					log.Printf("failed to load user %s for session: %v", *session.UserID, err)
					http.Error(w, "Authentication error", http.StatusInternalServerError)
					return
				}

				// Check if user is disabled
				if user.DisabledAt != nil {
					http.Error(w, "Account disabled", http.StatusUnauthorized)
					return
				}

				subject = user.PrincipalSubject()
			} else if session.ServiceAccountID != nil && *session.ServiceAccountID != "" {
				// Load service account
				sa, err := deps.ServiceAccounts.GetByID(ctx, *session.ServiceAccountID)
				if err != nil {
					log.Printf("failed to load service account %s for session: %v", *session.ServiceAccountID, err)
					http.Error(w, "Authentication error", http.StatusInternalServerError)
					return
				}

				// Check if service account is disabled
				if sa.Disabled {
					http.Error(w, "Account disabled", http.StatusUnauthorized)
					return
				}

				subject = fmt.Sprintf("sa:%s", sa.ClientID)
			} else {
				log.Printf("session %s has no user_id or service_account_id", session.ID)
				http.Error(w, "Invalid session", http.StatusUnauthorized)
				return
			}

			// Create synthetic JWT claims that the authn middleware expects
			// The authn middleware will process these claims and set up the principal
			syntheticClaims := map[string]interface{}{
				"sub": subject,
				"jti": session.ID, // Use session ID as JTI (won't be in revoked_jti table, that's OK)
			}

			// Store synthetic claims in context for authn middleware
			ctx = auth.SetClaimsContext(ctx, syntheticClaims)

			// Store token hash for session lookup by authn middleware
			ctx = auth.SetTokenHashContext(ctx, tokenHash)

			// Store session metadata
			sessionMeta := SessionMetadata{
				SessionID: session.ID,
				TokenHash: tokenHash,
				ExpiresAt: session.ExpiresAt,
			}
			ctx = context.WithValue(ctx, defaultSessionContextKey, sessionMeta)

			// Update last used timestamp (best effort, non-blocking)
			go func() {
				updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := deps.Sessions.UpdateLastUsed(updateCtx, session.ID); err != nil {
					log.Printf("warning: failed to update session last_used: %v", err)
				}
			}()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionMetadataFromContext retrieves session metadata from context
func SessionMetadataFromContext(ctx context.Context) (SessionMetadata, bool) {
	meta, ok := ctx.Value(defaultSessionContextKey).(SessionMetadata)
	return meta, ok
}

// shouldSkipSessionAuth determines if session authentication should be skipped for the request
func shouldSkipSessionAuth(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.Method == http.MethodOptions {
		return true
	}

	// Public paths that don't require session authentication
	// Note: /api/auth/whoami is NOT public - it requires authentication
	publicPaths := []string{
		"/health",
		"/auth/login",
		"/auth/sso/login",
		"/auth/sso/callback",
		"/auth/config",
		"/.well-known/",
		"/device_authorization",
		"/token",
		"/jwks",
		"/keys",
		"/authorization",
		"/oauth/",
	}

	for _, prefix := range publicPaths {
		if r.URL.Path == prefix || (len(r.URL.Path) > len(prefix) && r.URL.Path[:len(prefix)] == prefix) {
			return true
		}
	}

	return false
}
