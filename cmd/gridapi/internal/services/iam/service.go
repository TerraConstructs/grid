package iam

import (
	"context"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// Service provides all identity and access management operations.
//
// This service centralizes:
//   - Authentication (request path - performance critical)
//   - Authorization (request path - read-only Casbin)
//   - Session management (login/logout)
//   - User/service account management (admin operations)
//   - Role assignment (admin operations - triggers cache refresh)
//   - Cache management (out-of-band refresh)
type Service interface {
	// =========================================================================
	// Authentication (Request Path - Performance Critical)
	// =========================================================================

	// AuthenticateRequest tries all registered authenticators in order.
	// Returns the first successful Principal, or nil if none succeed.
	//
	// Authenticators are tried in priority order:
	//   1. SessionAuthenticator (checks cookie)
	//   2. JWTAuthenticator (checks Bearer token)
	//
	// Returns:
	//   - (principal, nil): Authentication successful
	//   - (nil, nil): No valid credentials found (unauthenticated request)
	//   - (nil, error): Authentication failed (invalid credentials)
	AuthenticateRequest(ctx context.Context, req AuthRequest) (*Principal, error)

	// ResolveRoles computes effective roles for a principal.
	//
	// This is a PURE FUNCTION with no side effects:
	//   - No database writes
	//   - No Casbin mutation
	//   - Uses immutable group→role cache for lock-free reads
	//
	// Parameters:
	//   - principalID: users.id or service_accounts.id (UUID)
	//   - groups: Group names from JWT/session
	//   - isUser: true for users, false for service accounts
	//
	// Returns: user_roles ∪ group_roles (union, deduplicated)
	ResolveRoles(ctx context.Context, principalID string, groups []string, isUser bool) ([]string, error)

	// =========================================================================
	// Authorization (Request Path - Read-Only)
	// =========================================================================

	// Authorize checks if principal has permission for action on object with labels.
	//
	// Uses principal.Roles (pre-resolved at authentication time) to check
	// against Casbin policies. NO Casbin state mutation occurs.
	//
	// For each role in principal.Roles:
	//   - Query Casbin policy: enforcer.Enforce(roleID, obj, act, labels)
	//   - If ANY role allows: return true
	//
	// This is READ-ONLY - no AddGroupingPolicy or similar calls.
	Authorize(ctx context.Context, principal *Principal, obj, act string, labels map[string]interface{}) (bool, error)

	// =========================================================================
	// Cache Management (Out-of-Band, Not in Request Path)
	// =========================================================================

	// RefreshGroupRoleCache reloads group→role mappings from database.
	//
	// Uses atomic.Value.Store() for zero-downtime hot-reload:
	//   1. Build new map from DB (group_roles + roles tables)
	//   2. Create immutable snapshot
	//   3. Atomically swap pointer (old readers unaffected)
	//
	// Called by:
	//   - Server startup (initial load)
	//   - Background goroutine (periodic refresh, e.g., every 5 minutes)
	//   - Admin API (manual refresh)
	//   - After AssignGroupRole/RemoveGroupRole (automatic refresh)
	RefreshGroupRoleCache(ctx context.Context) error

	// GetGroupRoleCacheSnapshot returns the current cache snapshot for debugging.
	// Contains: map[groupName][]roleName, version, timestamp
	GetGroupRoleCacheSnapshot() GroupRoleSnapshot

	// =========================================================================
	// Session Management (Login/Logout - Control Plane)
	// =========================================================================

	// CreateSession creates a new session after successful authentication.
	//
	// Parameters:
	//   - userID: users.id (UUID)
	//   - idToken: JWT from external IdP (stored for group extraction)
	//   - expiresAt: Session expiry time
	//
	// Returns:
	//   - session: Created session record
	//   - token: Unhashed session token (set as cookie)
	//   - error: If creation fails
	//
	// The token is hashed (SHA256) before storage for security.
	CreateSession(ctx context.Context, userID, idToken string, expiresAt time.Time) (*models.Session, string, error)

	// RevokeSession invalidates a session by ID.
	// Sets session.revoked = true and session.revoked_at = now().
	RevokeSession(ctx context.Context, sessionID string) error

	// GetSessionByID retrieves a session by its ID.
	// Returns repository.ErrNotFound if session doesn't exist.
	GetSessionByID(ctx context.Context, sessionID string) (*models.Session, error)

	// ListUserSessions retrieves all sessions for a specific user.
	// Returns empty slice if user has no active sessions.
	ListUserSessions(ctx context.Context, userID string) ([]models.Session, error)

	// RevokeJTI adds a JWT ID to the revocation list.
	// Used for logout and emergency token revocation.
	RevokeJTI(ctx context.Context, jti string, expiresAt time.Time) error

	// =========================================================================
	// User Management (Admin Operations)
	// =========================================================================

	// CreateUser creates a new user (internal or external IdP).
	//
	// Parameters:
	//   - email: User's email address (required)
	//   - username: Display name (required)
	//   - subject: OIDC subject from external IdP (optional, for Mode 1)
	//   - passwordHash: bcrypt password hash (optional, for Mode 2 internal users)
	//
	// Used by:
	//   - SSO callback handler (JIT provisioning with subject)
	//   - gridapi users create CLI command (internal users with passwordHash)
	CreateUser(ctx context.Context, email, username, subject, passwordHash string) (*models.User, error)

	// GetUserByEmail retrieves user by email address.
	// Returns repository.ErrNotFound if user doesn't exist.
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)

	// GetUserBySubject retrieves user by OIDC subject.
	// Returns repository.ErrNotFound if user doesn't exist.
	GetUserBySubject(ctx context.Context, subject string) (*models.User, error)

	// GetUserByID retrieves user by internal ID.
	// Returns repository.ErrNotFound if user doesn't exist.
	GetUserByID(ctx context.Context, userID string) (*models.User, error)

	// DisableUser sets user.disabled = true.
	// Disabled users cannot authenticate.
	DisableUser(ctx context.Context, userID string) error

	// =========================================================================
	// Service Account Management (Admin Operations)
	// =========================================================================

	// CreateServiceAccount creates a new service account for machine-to-machine auth.
	//
	// Returns:
	//   - serviceAccount: Created record
	//   - clientSecret: Unhashed secret (return to caller, not stored)
	//
	// The secret is hashed (bcrypt) before storage.
	CreateServiceAccount(ctx context.Context, name, createdBy string) (*models.ServiceAccount, string, error)

	// ListServiceAccounts returns all service accounts.
	ListServiceAccounts(ctx context.Context) ([]*models.ServiceAccount, error)

	// GetServiceAccountByName retrieves a service account by its human-readable name.
	// Returns repository.ErrNotFound if the service account doesn't exist.
	GetServiceAccountByName(ctx context.Context, name string) (*models.ServiceAccount, error)

	// GetServiceAccountByClientID retrieves a service account by its client ID.
	GetServiceAccountByClientID(ctx context.Context, clientID string) (*models.ServiceAccount, error)

	// GetServiceAccountByID retrieves a service account by its internal ID.
	// Returns repository.ErrNotFound if service account doesn't exist.
	GetServiceAccountByID(ctx context.Context, saID string) (*models.ServiceAccount, error)

	// RevokeServiceAccount disables a service account and cleans up all associated resources.
	// This is an out-of-band mutation operation that:
	// - Sets disabled = true in the database
	// - Revokes all active sessions for the service account
	// - Removes all Casbin role assignments for the service account
	RevokeServiceAccount(ctx context.Context, clientID string) error

	// RotateServiceAccountSecret generates a new secret for a service account.
	// Returns the unhashed secret (caller must save it) and the timestamp of rotation.
	// The secret is hashed with bcrypt before storage.
	RotateServiceAccountSecret(ctx context.Context, clientID string) (string, time.Time, error)

	// AssignRolesToServiceAccount assigns one or more roles to a service account.
	//
	// This method wraps AssignUserRole for convenience and ensures cache refresh
	// semantics remain centralized.
	AssignRolesToServiceAccount(ctx context.Context, serviceAccountID string, roleIDs []string) error

	// RemoveRolesFromServiceAccount removes one or more roles from a service account.
	RemoveRolesFromServiceAccount(ctx context.Context, serviceAccountID string, roleIDs []string) error

	// =========================================================================
	// Role Assignment (Admin Operations - Triggers Cache Refresh)
	// =========================================================================

	// AssignUserRole assigns a role directly to a user or service account.
	//
	// Exactly one of userID or serviceAccountID must be provided (non-empty string).
	// This creates a UserRole record and syncs to Casbin.
	//
	// This is an out-of-band mutation (admin operation), not part of the auth request path.
	AssignUserRole(ctx context.Context, userID, serviceAccountID, roleID string) error

	// RemoveUserRole removes a role from a user or service account.
	//
	// Exactly one of userID or serviceAccountID must be provided (non-empty string).
	// This deletes the UserRole record and removes from Casbin.
	//
	// This is an out-of-band mutation (admin operation), not part of the auth request path.
	RemoveUserRole(ctx context.Context, userID, serviceAccountID, roleID string) error

	// AssignGroupRole assigns a role to an IdP group.
	//
	// After persisting to database, automatically calls RefreshGroupRoleCache()
	// to ensure new mappings are visible to authentication flow immediately.
	//
	// This is a control-plane operation (not in request path), so the cache
	// refresh latency is acceptable.
	AssignGroupRole(ctx context.Context, groupName, roleID string) error

	// RemoveGroupRole removes a role from a group.
	//
	// After deletion, automatically calls RefreshGroupRoleCache().
	RemoveGroupRole(ctx context.Context, groupName, roleID string) error

	// =========================================================================
	// Role Management (Admin Operations - CRUD for Roles)
	// =========================================================================

	// CreateRole creates a new role with permissions synced to Casbin.
	//
	// This is an out-of-band mutation operation that:
	//   1. Validates label_scope_expr as valid go-bexpr syntax
	//   2. Creates a Role record in the database
	//   3. Parses actions (format "obj:act") and adds Casbin policies
	//   4. Rolls back the database change if Casbin sync fails
	//
	// Parameters:
	//   - name: Role name (e.g., "platform-engineer")
	//   - description: Human-readable description
	//   - scopeExpr: Label scope expression (go-bexpr syntax, e.g., "env == 'prod'")
	//   - createConstraints: Map of label key → constraint (allowed values, required)
	//   - immutableKeys: List of label keys that cannot be changed
	//   - actions: List of actions in "obj:act" format (e.g., ["state:read", "state:write"])
	//
	// Returns the created role with generated ID, or error if validation/creation fails.
	CreateRole(
		ctx context.Context,
		name, description, scopeExpr string,
		createConstraints models.CreateConstraints,
		immutableKeys []string,
		actions []string,
	) (*models.Role, error)

	// UpdateRole updates an existing role's permissions and metadata.
	//
	// This is an out-of-band mutation operation that:
	//   1. Validates label_scope_expr as valid go-bexpr syntax
	//   2. Checks optimistic locking (version must match)
	//   3. Updates the Role record in the database
	//   4. Removes all old Casbin policies for the role
	//   5. Adds new Casbin policies based on updated actions
	//
	// Parameters:
	//   - name: Role name (immutable, used for lookup)
	//   - expectedVersion: For optimistic locking (must match current version)
	//   - description, scopeExpr, createConstraints, immutableKeys, actions: Same as CreateRole
	//
	// Returns the updated role with incremented version, or error if validation/update fails.
	// Returns error if version mismatch (concurrent modification detected).
	UpdateRole(
		ctx context.Context,
		name string,
		expectedVersion int,
		description, scopeExpr string,
		createConstraints models.CreateConstraints,
		immutableKeys []string,
		actions []string,
	) (*models.Role, error)

	// DeleteRole deletes a role and removes all associated Casbin policies.
	//
	// This is an out-of-band mutation operation that:
	//   1. Verifies the role exists
	//   2. Checks if the role is assigned to any principals (safety check)
	//   3. Deletes the Role record from the database
	//   4. Removes all Casbin policies for the role
	//
	// Safety: Rejects deletion if role is assigned to any principals.
	// Returns error if role not found, still assigned, or deletion fails.
	DeleteRole(ctx context.Context, name string) error

	// =========================================================================
	// Read-Only Lookup Methods (For Handlers - No Mutations)
	// =========================================================================

	// GetRoleByName retrieves a role by its name.
	// Returns repository.ErrNotFound if role doesn't exist.
	GetRoleByName(ctx context.Context, name string) (*models.Role, error)

	// GetRolesByName returns the roles matching the provided names along with metadata
	// about invalid inputs and the complete set of valid role names.
	//
	// The returned roles preserve the order of roleNames and only include duplicates
	// if the input slice does. Invalid names are surfaced so callers can craft
	// contextual error messages without re-querying repositories.
	GetRolesByName(ctx context.Context, roleNames []string) ([]models.Role, []string, []string, error)

	// GetRoleByID retrieves a role by its internal ID.
	// Returns repository.ErrNotFound if role doesn't exist.
	GetRoleByID(ctx context.Context, roleID string) (*models.Role, error)

	// ListAllRoles returns all roles in the system.
	// Returns empty slice if no roles exist.
	ListAllRoles(ctx context.Context) ([]models.Role, error)

	// ListGroupRoles returns all group→role assignments, optionally filtered by group name.
	// If groupName is nil, returns all assignments.
	// If groupName is non-nil, returns only assignments for that group.
	ListGroupRoles(ctx context.Context, groupName *string) ([]models.GroupRole, error)

	// GetPrincipalRoles returns the Casbin role IDs for a principal.
	// This replaces direct Enforcer.GetRolesForUser() calls in handlers.
	//
	// Parameters:
	//   - principalID: users.id or service_accounts.id (UUID)
	//   - principalType: "user" or "service_account"
	//
	// Returns: Array of Casbin role IDs (e.g., ["role::platform-engineer"]) with auth prefix.
	GetPrincipalRoles(ctx context.Context, principalID, principalType string) ([]string, error)

	// GetRolePermissions returns the Casbin permissions for a role.
	// This replaces direct Enforcer.GetPermissionsForUser() calls in handlers.
	//
	// Parameters:
	//   - roleName: Role name (e.g., "platform-engineer")
	//
	// Returns: Array of permission tuples from Casbin (e.g., [["role::platform-engineer", "state", "read", ...]]).
	GetRolePermissions(ctx context.Context, roleName string) ([][]string, error)
}

// GroupRoleSnapshot is an immutable snapshot of group→role mappings.
//
// Stored in atomic.Value for lock-free reads. Never modified after creation.
// To update, create a new snapshot and atomically swap the pointer.
type GroupRoleSnapshot struct {
	// Mappings: groupName → []roleName
	// Example: {"platform-engineers": ["platform-engineer"], "dev-team": ["product-engineer"]}
	Mappings map[string][]string

	// CreatedAt is when this snapshot was built.
	CreatedAt time.Time

	// Version is an incrementing counter for debugging.
	// Helps identify which snapshot is active.
	Version int
}
