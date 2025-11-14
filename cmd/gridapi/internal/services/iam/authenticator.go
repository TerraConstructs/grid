package iam

import (
	"context"
	"net/http"
)

// Authenticator validates credentials and returns a Principal with resolved roles.
//
// Implementations:
//   - JWTAuthenticator: Validates Bearer tokens (internal or external IdP)
//   - SessionAuthenticator: Validates session cookies
//
// Return values:
//   - (principal, nil): Authentication successful
//   - (nil, nil): Credentials not present (not an error, try next authenticator)
//   - (nil, error): Authentication failed (invalid credentials)
//
// The authenticator is responsible for:
//   1. Extracting credentials from request
//   2. Validating credentials (signature, expiry, revocation)
//   3. Resolving identity (user or service account)
//   4. Computing effective roles (user_roles ∪ group_roles)
//   5. Constructing immutable Principal struct
type Authenticator interface {
	// Authenticate validates credentials and returns a Principal with resolved roles.
	Authenticate(ctx context.Context, req AuthRequest) (*Principal, error)
}

// AuthRequest wraps HTTP request data for authenticator implementations.
// This abstraction allows authenticators to work with both HTTP and Connect RPC requests.
type AuthRequest struct {
	// Headers contains HTTP headers (including Authorization, Cookie)
	Headers http.Header

	// Cookies contains parsed cookies
	Cookies []*http.Cookie
}
