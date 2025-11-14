package iam

// Principal represents an authenticated identity with pre-resolved roles.
//
// This struct is IMMUTABLE after construction. Roles are computed once at
// authentication time and never modified. This eliminates race conditions
// caused by shared mutable state.
//
// The Principal is stored in request context and used by authorization
// middleware to make access control decisions.
type Principal struct {
	// Subject is the stable OIDC subject or client identifier (unprefixed).
	// Examples: "alice@example.com", "sa:019a1234-5678-7abc-def0-123456789abc"
	Subject string

	// PrincipalID is the Casbin-ready identifier with type prefix.
	// Examples: "user:alice@example.com", "sa:019a1234-..."
	// Used for Casbin policy evaluation.
	PrincipalID string

	// InternalID references the backing database record.
	// For users: users.id (UUID)
	// For service accounts: service_accounts.id (UUID)
	InternalID string

	// Email is the user's email address (optional, only for human users).
	Email string

	// Name is the display name (optional, only for human users).
	Name string

	// SessionID references the active session (optional, only for cookie auth).
	// References sessions.id (UUID).
	SessionID string

	// Groups lists the IdP groups this principal belongs to.
	// Examples: ["platform-engineers", "product-engineers"]
	// Source: JWT claims (external IdP) or session.id_token (stored JWT).
	Groups []string

	// Roles lists the resolved role names (NOT Casbin role IDs).
	// Examples: ["platform-engineer", "product-engineer"]
	// Computed as: user_roles ∪ group_roles (via immutable cache).
	//
	// This is the SOURCE OF TRUTH for authorization decisions.
	// Authorization checks iterate over these roles and query Casbin
	// policies without mutating any state.
	Roles []string

	// Type differentiates users and service accounts.
	Type PrincipalType
}

// PrincipalType identifies whether this is a user or service account.
type PrincipalType string

const (
	// PrincipalTypeUser represents a human user (authenticated via password or SSO).
	PrincipalTypeUser PrincipalType = "user"

	// PrincipalTypeServiceAccount represents a machine identity (authenticated via client credentials).
	PrincipalTypeServiceAccount PrincipalType = "service_account"
)
