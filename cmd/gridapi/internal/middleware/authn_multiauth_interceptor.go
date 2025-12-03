package middleware

import (
	"context"
	"log"

	"connectrpc.com/connect"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
)

// NewMultiAuthInterceptor is the Phase 3 unified authentication interceptor for Connect RPC.
//
// This interceptor:
//  1. Extracts headers and metadata from Connect request
//  2. Calls iamService.AuthenticateRequest() which tries all authenticators
//  3. Sets Principal in context if authentication succeeds
//  4. Continues to next handler (authentication failure handled by authz)
//
// Authentication flow:
//   - SessionAuthenticator checks grid.session cookie (via Connect metadata)
//   - JWTAuthenticator checks Authorization: Bearer header
//   - First successful authenticator wins
//   - If all return (nil, nil): unauthenticated request (allowed)
//   - If any returns (nil, error): authentication failed (401)
//
// This replaces the old session_interceptor.go and jwt_interceptor.go which had
// scattered authentication logic and Casbin mutation.
func NewMultiAuthInterceptor(iamService iam.Service) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			// Step 1: Build AuthRequest from Connect request
			// Connect metadata includes both headers and cookies
			authReq := iam.AuthRequest{
				Headers: req.Header(),
				// Note: Connect doesn't expose cookies directly, they're in headers
				// The authenticators will extract from Cookie header if needed
				Cookies: nil,
			}

			// Step 2: Try all authenticators via IAM service
			principal, err := iamService.AuthenticateRequest(ctx, authReq)
			if err != nil {
				// Authentication failed (invalid credentials)
				log.Printf("authentication failed for procedure %s: %v", req.Spec().Procedure, err)
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
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

			// Step 4: Continue to next handler/interceptor
			// Note: Unauthenticated requests (principal == nil) are allowed here.
			// The authorization interceptor will enforce permission checks.
			return next(ctx, req)
		})
	})
}
