# Phase 1: Services Layer Foundation

**Priority**: P0 (Critical path)
**Effort**: 6-8 hours
**Risk**: Low
**Dependencies**: None

## Objectives

- Establish proper directory structure for IAM service layer
- Define core interfaces (Authenticator, Service)
- Define Principal struct (normalized authentication result)
- Create scaffolding for implementation in later phases

## Deliverables

- [ ] `internal/services/iam/` directory created
- [ ] Authenticator interface defined
- [ ] Principal struct defined
- [ ] IAM Service interface defined
- [ ] Package compiles successfully
- [ ] Basic package documentation written

## Directory Structure

```
cmd/gridapi/internal/services/iam/
├── authenticator.go      # Authenticator interface
├── principal.go          # Principal struct
├── service.go            # Service interface
└── doc.go                # Package documentation
```

## Tasks

### Task 1.1: Create IAM Package Structure

**Effort**: 30 minutes

Create directory and package documentation:

```bash
mkdir -p cmd/gridapi/internal/services/iam
```

**File**: `cmd/gridapi/internal/services/iam/doc.go`

```go
// Package iam provides identity and access management services for Grid API.
//
// The IAM service centralizes all authentication, authorization, session management,
// and role resolution logic. It provides:
//
//   - Authentication via multiple strategies (JWT, Session cookies)
//   - Immutable group→role cache with lock-free reads
//   - Read-only Casbin policy evaluation
//   - Session lifecycle management
//   - User and service account management
//
// Architecture:
//
//   - Authenticator interface: Pluggable authentication strategies
//   - Principal struct: Unified authentication result (immutable)
//   - GroupRoleCache: Atomic snapshot cache for group→role mappings
//   - Service interface: Facade for all IAM operations
//
// Request Flow:
//
//	Request → MultiAuth → Authenticator.Authenticate() → Principal (with Roles)
//	       ↓
//	   Handler → IAM.Authorize(principal) → Casbin (read-only)
//
// The key design principle is that roles are resolved ONCE at authentication time
// and stored in the Principal struct. Authorization then uses these pre-resolved roles
// without mutating any shared state.
package iam
```

### Task 1.2: Define Authenticator Interface

**Effort**: 1 hour

**File**: `cmd/gridapi/internal/services/iam/authenticator.go`

```go
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
```

**Acceptance Criteria**:
- [ ] Interface compiles
- [ ] Godoc comments clear and complete
- [ ] Return value semantics documented (nil vs error)

### Task 1.3: Define Principal Struct

**Effort**: 1 hour

**File**: `cmd/gridapi/internal/services/iam/principal.go`

```go
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
```

**Acceptance Criteria**:
- [ ] Struct compiles
- [ ] All fields documented with examples
- [ ] Immutability emphasized in documentation
- [ ] Compatible with existing `auth.AuthenticatedPrincipal` (for migration)

### Task 1.4: Define IAM Service Interface

**Effort**: 2-3 hours

**File**: `cmd/gridapi/internal/services/iam/service.go`

```go
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
	// Returns: user_roles ∪ group_roles (union, deduplicated)
	ResolveRoles(ctx context.Context, userID string, groups []string) ([]string, error)

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

	// RevokeJTI adds a JWT ID to the revocation list.
	// Used for logout and emergency token revocation.
	RevokeJTI(ctx context.Context, jti string, expiresAt time.Time) error

	// =========================================================================
	// User Management (Admin Operations)
	// =========================================================================

	// CreateUser creates a new internal user (Mode 2 - internal IdP).
	//
	// Used by:
	//   - gridapi users create CLI command
	//   - POST /auth/login (JIT provisioning for external IdP)
	//
	// Password must be pre-hashed with bcrypt before calling.
	CreateUser(ctx context.Context, email, username, passwordHash string) (*models.User, error)

	// GetUserByEmail retrieves user by email address.
	// Returns repository.ErrNotFound if user doesn't exist.
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)

	// GetUserBySubject retrieves user by OIDC subject.
	// Returns repository.ErrNotFound if user doesn't exist.
	GetUserBySubject(ctx context.Context, subject string) (*models.User, error)

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

	// =========================================================================
	// Role Assignment (Admin Operations - Triggers Cache Refresh)
	// =========================================================================

	// AssignUserRole assigns a role directly to a user.
	// Persists to user_roles table.
	AssignUserRole(ctx context.Context, userID, roleID string) error

	// RemoveUserRole removes a role from a user.
	// Deletes from user_roles table.
	RemoveUserRole(ctx context.Context, userID, roleID string) error

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
```

**Acceptance Criteria**:
- [ ] Interface compiles
- [ ] All methods documented with parameters, return values, examples
- [ ] Performance-critical paths identified in comments
- [ ] Cache refresh semantics clear
- [ ] Compatible with existing middleware (for migration)

## Testing Strategy

No tests in this phase - interfaces only. Testing comes in implementation phases.

## Migration Notes

This phase is **additive only** - no existing code is modified. The new interfaces will be implemented in later phases, then existing code will be refactored to use them.

## Related Documents

- **Next Phase**: [phase-2-immutable-cache.md](phase-2-immutable-cache.md)
- **Architecture**: [architecture-analysis.md](architecture-analysis.md)
