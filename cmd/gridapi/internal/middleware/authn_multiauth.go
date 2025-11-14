package middleware

import (
	"log"
	"net/http"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
)

// MultiAuthMiddleware is the Phase 3 unified authentication middleware.
//
// This middleware:
//   1. Extracts headers and cookies from HTTP request
//   2. Calls iamService.AuthenticateRequest() which tries all authenticators
//   3. Sets Principal in context if authentication succeeds
//   4. Continues to next handler (authentication failure handled by authz)
//
// Authentication flow:
//   - SessionAuthenticator checks grid.session cookie
//   - JWTAuthenticator checks Authorization: Bearer header
//   - First successful authenticator wins
//   - If all return (nil, nil): unauthenticated request (allowed)
//   - If any returns (nil, error): authentication failed (401)
//
// This replaces the old authn.go middleware which had 7 steps with database
// queries and Casbin mutation. The new flow delegates everything to the
// IAM service which uses the immutable cache for lock-free role resolution.
func MultiAuthMiddleware(iamService iam.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Step 1: Build AuthRequest from HTTP request
			authReq := iam.AuthRequest{
				Headers: r.Header,
				Cookies: r.Cookies(),
			}

			// Step 2: Try all authenticators via IAM service
			principal, err := iamService.AuthenticateRequest(ctx, authReq)
			if err != nil {
				// Authentication failed (invalid credentials)
				log.Printf("authentication failed for %s %s: %v", r.Method, r.URL.Path, err)
				http.Error(w, "authentication failed", http.StatusUnauthorized)
				return
			}

			// Step 3: Set Principal and Groups in context
			if principal != nil {
				// Convert iam.Principal to auth.AuthenticatedPrincipal for legacy compatibility
				legacyPrincipal := auth.AuthenticatedPrincipal{
					Subject:     principal.Subject,
					PrincipalID: principal.PrincipalID,
					InternalID:  principal.InternalID,
					Email:       principal.Email,
					Name:        principal.Name,
					SessionID:   principal.SessionID,
					Roles:       principal.Roles,
					Type:        auth.PrincipalType(principal.Type),
				}

				ctx = auth.SetUserContext(ctx, legacyPrincipal)
				ctx = auth.SetGroupsContext(ctx, principal.Groups)
			}

			// Step 4: Continue to next handler
			// Note: Unauthenticated requests (principal == nil) are allowed here.
			// The authorization middleware will enforce permission checks.
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
