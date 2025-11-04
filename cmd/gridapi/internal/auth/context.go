package auth

import "context"

// PrincipalType describes the type of authenticated principal.
type PrincipalType string

const (
	// PrincipalTypeUser represents a human user authenticated via OIDC.
	PrincipalTypeUser PrincipalType = "user"
	// PrincipalTypeServiceAccount represents a non-interactive service account.
	PrincipalTypeServiceAccount PrincipalType = "service_account"
)

// AuthenticatedPrincipal captures identity metadata propagated through the request context.
type AuthenticatedPrincipal struct {
	// Subject is the stable OIDC subject or client identifier (unprefixed).
	Subject string
	// PrincipalID is the Casbin-ready identifier (e.g., user:alice@example.com).
	PrincipalID string
	// InternalID references the backing database record (users.id or service_accounts.id).
	InternalID string
	// Email is optional and present for human users when available.
	Email string
	// Name is optional display name for human users.
	Name string
	// SessionID references the active session row when available.
	SessionID string
	// Roles lists effective Casbin role identifiers resolved during authentication.
	Roles []string
	// Type differentiates users and service accounts.
	Type PrincipalType
}

type principalContextKey struct{}

// SetUserContext stores the authenticated principal on the context for downstream consumers.
func SetUserContext(ctx context.Context, principal AuthenticatedPrincipal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// GetUserFromContext retrieves the authenticated principal from the context.
func GetUserFromContext(ctx context.Context) (AuthenticatedPrincipal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(AuthenticatedPrincipal)
	return principal, ok
}

type groupsContextKey struct{}

// SetGroupsContext stores the resolved group list on the context.
func SetGroupsContext(ctx context.Context, groups []string) context.Context {
	copied := append([]string(nil), groups...)
	return context.WithValue(ctx, groupsContextKey{}, copied)
}

// GetGroupsFromContext retrieves the resolved group list from the context.
func GetGroupsFromContext(ctx context.Context) []string {
	groups, ok := ctx.Value(groupsContextKey{}).([]string)
	if !ok {
		return nil
	}
	return append([]string(nil), groups...)
}
