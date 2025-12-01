package iam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/hashicorp/go-bexpr"
	"go.opentelemetry.io/otel/attribute"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/telemetry"
	"golang.org/x/crypto/bcrypt"
)

// iamService implements the Service interface.
//
// This is the concrete implementation that will be used throughout the gridapi
// codebase. It coordinates between repositories, the immutable cache, Casbin
// enforcer, and authenticator implementations.
type iamService struct {
	// Repositories
	users           repository.UserRepository
	serviceAccounts repository.ServiceAccountRepository
	sessions        repository.SessionRepository
	userRoles       repository.UserRoleRepository
	groupRoles      repository.GroupRoleRepository
	roles           repository.RoleRepository
	revokedJTIs     repository.RevokedJTIRepository

	// Immutable cache (lock-free reads)
	groupRoleCache *GroupRoleCache

	// Casbin enforcer (read-only for authorization)
	enforcer casbin.IEnforcer

	// Authenticators (injected, populated in Phase 3)
	authenticators []Authenticator
}

// IAMServiceDependencies contains all dependencies for IAM service construction.
//
// This struct is used for dependency injection, making it easy to:
//   - Test with mocks
//   - Swap implementations
//   - Add new dependencies without breaking existing code
type IAMServiceDependencies struct {
	Users           repository.UserRepository
	ServiceAccounts repository.ServiceAccountRepository
	Sessions        repository.SessionRepository
	UserRoles       repository.UserRoleRepository
	GroupRoles      repository.GroupRoleRepository
	Roles           repository.RoleRepository
	RevokedJTIs     repository.RevokedJTIRepository
	Enforcer        casbin.IEnforcer
}

// IAMServiceConfig contains configuration for IAM service construction.
// Separated from dependencies to clearly distinguish config from runtime dependencies.
type IAMServiceConfig struct {
	Config *config.Config
}

// NewIAMService creates a new IAM service with all dependencies.
//
// This constructor:
//   - Initializes the GroupRoleCache with initial load from database
//   - Returns error if initial cache load fails (server cannot start)
//   - Initializes authenticators (SessionAuthenticator + JWTAuthenticator)
//
// The cache initialization is critical - the server must not start if
// the cache cannot be loaded, as role resolution would fail for all requests.
func NewIAMService(deps IAMServiceDependencies, cfg IAMServiceConfig) (Service, error) {
	// Initialize cache with initial load
	cache, err := NewGroupRoleCache(deps.GroupRoles, deps.Roles)
	if err != nil {
		return nil, fmt.Errorf("initialize group role cache: %w", err)
	}

	// Create service instance (without authenticators yet)
	svc := &iamService{
		users:           deps.Users,
		serviceAccounts: deps.ServiceAccounts,
		sessions:        deps.Sessions,
		userRoles:       deps.UserRoles,
		groupRoles:      deps.GroupRoles,
		roles:           deps.Roles,
		revokedJTIs:     deps.RevokedJTIs,
		groupRoleCache:  cache,
		enforcer:        deps.Enforcer,
		authenticators:  []Authenticator{}, // Initialized below
	}

	// Phase 3: Initialize authenticators
	authenticators, err := initializeAuthenticators(cfg.Config, deps, svc)
	if err != nil {
		return nil, fmt.Errorf("initialize authenticators: %w", err)
	}
	svc.authenticators = authenticators

	return svc, nil
}

// initializeAuthenticators creates and registers authenticators.
//
// Authenticator priority:
//  1. SessionAuthenticator (checks grid.session cookie)
//  2. JWTAuthenticator (checks Authorization: Bearer header)
//
// Returns empty slice if auth is disabled (cfg.OIDC not configured).
func initializeAuthenticators(
	cfg *config.Config,
	deps IAMServiceDependencies,
	svc *iamService,
) ([]Authenticator, error) {
	var authenticators []Authenticator

	// SessionAuthenticator always available (even if OIDC disabled)
	sessionAuth := NewSessionAuthenticator(
		deps.Users,
		deps.Sessions,
		svc,
	)
	authenticators = append(authenticators, sessionAuth)

	// JWTAuthenticator only if OIDC configured
	jwtAuth, err := NewJWTAuthenticator(
		cfg,
		deps.Users,
		deps.ServiceAccounts,
		deps.RevokedJTIs,
		svc,
	)
	if err != nil {
		return nil, fmt.Errorf("create JWT authenticator: %w", err)
	}
	if jwtAuth != nil {
		authenticators = append(authenticators, jwtAuth)
	}

	return authenticators, nil
}

// =========================================================================
// Authentication (Request Path - Performance Critical)
// =========================================================================

// AuthenticateRequest tries all registered authenticators in order.
//
// Authenticator priority (from Phase 3 spec):
//  1. SessionAuthenticator (checks grid.session cookie)
//  2. JWTAuthenticator (checks Authorization: Bearer header)
//
// Algorithm:
//   - Try each authenticator in sequence
//   - If authenticator returns (nil, nil): no credentials, try next
//   - If authenticator returns (nil, error): authentication failed, stop and return error
//   - If authenticator returns (principal, nil): success, stop and return principal
//   - If all authenticators return (nil, nil): return (nil, nil) for unauthenticated request
func (s *iamService) AuthenticateRequest(ctx context.Context, req AuthRequest) (*Principal, error) {
	ctx, span := telemetry.StartSpan(ctx, "gridapi/services/iam", "iam.AuthenticateRequest",
		attribute.Int("authenticator_count", len(s.authenticators)),
	)
	defer span.End()

	for i, authenticator := range s.authenticators {
		principal, err := authenticator.Authenticate(ctx, req)
		if err != nil {
			// Authentication failed (invalid credentials)
			telemetry.AddEvent(span, "authentication.failed",
				attribute.Int("authenticator_index", i),
				attribute.String("error", err.Error()),
			)
			telemetry.RecordError(span, err)
			return nil, err
		}
		if principal != nil {
			// Authentication succeeded
			span.SetAttributes(
				attribute.String(telemetry.AttrPrincipalID, principal.ID),
				attribute.String(telemetry.AttrPrincipalType, string(principal.Type)),
				attribute.Int("authenticator_index", i),
			)
			telemetry.AddEvent(span, "authentication.succeeded",
				attribute.String("principal_id", principal.ID),
				attribute.Int("authenticator_index", i),
			)
			return principal, nil
		}
		// principal == nil && err == nil: no credentials for this authenticator, try next
	}

	// No valid credentials found (unauthenticated request)
	telemetry.AddEvent(span, "authentication.no_credentials")
	return nil, nil
}

// ResolveRoles computes effective roles for a principal.
//
// This is a PURE FUNCTION with no side effects. It:
//  1. Fetches principal's directly-assigned roles from database (1-2 DB queries)
//  2. Fetches group roles from IMMUTABLE CACHE (zero DB queries, lock-free)
//  3. Unions the two sets and deduplicates
//
// Performance characteristics:
//   - Before: 9 DB queries + mutex contention + Casbin mutation
//   - After: 2 DB queries + zero contention + zero mutation
//   - Expected latency: <10ms (down from 50-100ms)
func (s *iamService) ResolveRoles(ctx context.Context, principalID string, groups []string, isUser bool) ([]string, error) {
	ctx, span := telemetry.StartSpan(ctx, "gridapi/services/iam", "iam.ResolveRoles",
		attribute.String(telemetry.AttrPrincipalID, principalID),
		attribute.Bool("is_user", isUser),
		attribute.Int("group_count", len(groups)),
	)
	defer span.End()

	roleSet := make(map[string]struct{})

	// Step 1: Get principal's directly-assigned roles (DB read)
	var roleAssignments []models.UserRole
	var err error

	if isUser {
		roleAssignments, err = s.userRoles.GetByUserID(ctx, principalID)
	} else {
		roleAssignments, err = s.userRoles.GetByServiceAccountID(ctx, principalID)
	}

	if err != nil {
		telemetry.RecordError(span, err)
		return nil, fmt.Errorf("get principal roles: %w", err)
	}

	// Add directly assigned roles to set
	for _, assignment := range roleAssignments {
		role, err := s.roles.GetByID(ctx, assignment.RoleID)
		if err != nil {
			telemetry.RecordError(span, err)
			return nil, fmt.Errorf("get role %s: %w", assignment.RoleID, err)
		}
		roleSet[role.Name] = struct{}{}
	}

	span.SetAttributes(attribute.Int("direct_role_count", len(roleSet)))

	// Step 2: Get roles from groups (LOCK-FREE cache read)
	groupRoles := s.groupRoleCache.GetRolesForGroups(groups)
	for _, role := range groupRoles {
		roleSet[role] = struct{}{}
	}

	// Step 3: Convert to slice
	result := make([]string, 0, len(roleSet))
	for role := range roleSet {
		result = append(result, role)
	}

	return result, nil
}

// =========================================================================
// Authorization (Request Path - Read-Only)
// =========================================================================

// Authorize checks if principal has permission for action on object with labels.
//
// This is the Phase 4 implementation using read-only Casbin authorization.
// Uses principal.Roles (pre-resolved at authentication time) to check against
// Casbin policies. NO Casbin state mutation occurs.
//
// Delegation to AuthorizeWithRoles ensures:
//   - Zero lock contention (no global mutex)
//   - Zero Casbin mutation (no AddGroupingPolicy)
//   - Zero database writes
//   - Thread-safe concurrent calls
func (s *iamService) Authorize(ctx context.Context, principal *Principal, obj, act string, labels map[string]interface{}) (bool, error) {
	if principal == nil {
		return false, fmt.Errorf("nil principal")
	}

	// Use AuthorizeWithRoles from casbin_readonly.go
	return AuthorizeWithRoles(s.enforcer, principal.Roles, obj, act, labels)
}

// =========================================================================
// Cache Management (Out-of-Band, Not in Request Path)
// =========================================================================

// RefreshGroupRoleCache reloads group→role mappings from database.
//
// This delegates to the cache's Refresh method, which:
//  1. Builds new map from DB (on stack, not visible to readers)
//  2. Creates immutable snapshot
//  3. Atomically swaps pointer (zero downtime)
//
// Safe to call during request processing - readers will see either old or
// new snapshot atomically, never a partial update.
func (s *iamService) RefreshGroupRoleCache(ctx context.Context) error {
	return s.groupRoleCache.Refresh(ctx)
}

// GetGroupRoleCacheSnapshot returns the current cache snapshot for debugging.
//
// Returns a copy of the snapshot (not a pointer) to prevent callers from
// accidentally mutating the cache.
func (s *iamService) GetGroupRoleCacheSnapshot() GroupRoleSnapshot {
	snapshot := s.groupRoleCache.Get()
	if snapshot == nil {
		return GroupRoleSnapshot{
			Mappings:  make(map[string][]string),
			CreatedAt: time.Time{},
			Version:   0,
		}
	}
	return *snapshot
}

// =========================================================================
// Session Management (Login/Logout - Control Plane)
// =========================================================================

// CreateSession creates a new session after successful authentication.
//
// Generates a cryptographically secure session token, hashes it with SHA256,
// and stores the session record in the database. Returns the unhashed token
// (to be set as cookie) and the session record.
func (s *iamService) CreateSession(ctx context.Context, userID, idToken string, expiresAt time.Time) (*models.Session, string, error) {
	// Generate cryptographically secure session token (32 bytes = 64 hex chars)
	token, err := generateSessionToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate session token: %w", err)
	}

	// Hash token with SHA256 for storage
	tokenHash := hashToken(token)

	// Create session record
	session := &models.Session{
		UserID:    &userID,
		TokenHash: tokenHash,
		IDToken:   idToken,
		ExpiresAt: expiresAt,
	}

	// Persist to database
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, "", fmt.Errorf("create session: %w", err)
	}

	// Return session record and unhashed token (for cookie)
	return session, token, nil
}

// RevokeSession invalidates a session by ID.
//
// Sets session.revoked = true and session.revoked_at = now() in the database.
// Subsequent authentication attempts with this session will fail.
func (s *iamService) RevokeSession(ctx context.Context, sessionID string) error {
	if err := s.sessions.Revoke(ctx, sessionID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// GetSessionByID retrieves a session by its ID.
//
// Returns repository.ErrNotFound if the session doesn't exist.
func (s *iamService) GetSessionByID(ctx context.Context, sessionID string) (*models.Session, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return session, nil
}

// ListUserSessions retrieves all sessions for a specific user.
//
// Returns empty slice if user has no active sessions.
func (s *iamService) ListUserSessions(ctx context.Context, userID string) ([]models.Session, error) {
	sessions, err := s.sessions.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user sessions: %w", err)
	}
	return sessions, nil
}

// RevokeJTI adds a JWT ID to the revocation list.
//
// Implementation note: This is a stub. Will be implemented when JWT
// validation is added.
func (s *iamService) RevokeJTI(ctx context.Context, jti string, expiresAt time.Time) error {
	// Phase 3: Implement JTI revocation
	return fmt.Errorf("not implemented")
}

// =========================================================================
// User Management (Admin Operations)
// =========================================================================

// CreateUser creates a new user (internal or external IdP).
//
// For external IdP (Mode 1):
//   - email: User's email from ID token
//   - username: Display name from ID token
//   - subject: OIDC subject (e.g., "keycloak|123")
//   - passwordHash: "" (empty, not used)
//
// For internal IdP (Mode 2):
//   - email: User's email
//   - username: Display name
//   - subject: "" (empty, no external IdP)
//   - passwordHash: bcrypt hash of password
func (s *iamService) CreateUser(ctx context.Context, email, username, subject, passwordHash string) (*models.User, error) {
	// Create user model
	user := &models.User{
		Email: email,
		Name:  username,
	}

	// Set subject if provided (external IdP mode)
	if subject != "" {
		user.Subject = &subject
	}

	// Set password hash if provided (internal IdP mode)
	if passwordHash != "" {
		user.PasswordHash = &passwordHash
	}

	// Persist to database
	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves user by email address.
//
// Implementation note: Simple delegation to repository.
func (s *iamService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.users.GetByEmail(ctx, email)
}

// GetUserBySubject retrieves user by OIDC subject.
//
// Implementation note: Simple delegation to repository.
func (s *iamService) GetUserBySubject(ctx context.Context, subject string) (*models.User, error) {
	return s.users.GetBySubject(ctx, subject)
}

// GetUserByID retrieves user by internal ID.
//
// Returns repository.ErrNotFound if the user doesn't exist.
func (s *iamService) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// DisableUser sets user.disabled = true.
//
// Implementation note: This is a stub. Will be implemented in later phases.
func (s *iamService) DisableUser(ctx context.Context, userID string) error {
	// Later phase: Implement user disable
	return fmt.Errorf("not implemented")
}

// =========================================================================
// Service Account Management (Admin Operations)
// =========================================================================

// CreateServiceAccount creates a new service account.
//
// Generates client_id (UUIDv7), client_secret (32 random bytes), hashes the secret
// with bcrypt, and persists to database. Returns the service account record and
// the unhashed secret (caller must save it - it won't be shown again).
func (s *iamService) CreateServiceAccount(ctx context.Context, name, createdBy string) (*models.ServiceAccount, string, error) {
	// Generate client_id (UUIDv7 for time-sortable IDs)
	clientID := bunx.NewUUIDv7()

	// Generate client_secret (32 random bytes = 64 hex characters)
	clientSecret, err := generateSessionToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate client secret: %w", err)
	}

	// Hash secret with bcrypt (cost 10 is bcrypt.DefaultCost)
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash client secret: %w", err)
	}

	// Create service account record
	sa := &models.ServiceAccount{
		Name:             name,
		ClientID:         clientID,
		ClientSecretHash: string(hashedSecret),
		CreatedBy:        createdBy,
	}

	// Persist to database
	if err := s.serviceAccounts.Create(ctx, sa); err != nil {
		return nil, "", fmt.Errorf("create service account in database: %w", err)
	}

	return sa, clientSecret, nil
}

// ListServiceAccounts returns all service accounts.
//
// Implementation note: Simple delegation to repository.
func (s *iamService) ListServiceAccounts(ctx context.Context) ([]*models.ServiceAccount, error) {
	// Delegate to repository
	accounts, err := s.serviceAccounts.List(ctx)
	if err != nil {
		return nil, err
	}

	// Convert []models.ServiceAccount to []*models.ServiceAccount
	result := make([]*models.ServiceAccount, len(accounts))
	for i := range accounts {
		result[i] = &accounts[i]
	}

	return result, nil
}

// GetServiceAccountByClientID retrieves a service account by its client ID.
//
// Implementation note: Simple delegation to repository.
func (s *iamService) GetServiceAccountByClientID(ctx context.Context, clientID string) (*models.ServiceAccount, error) {
	sa, err := s.serviceAccounts.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("get service account by client ID: %w", err)
	}
	return sa, nil
}

// GetServiceAccountByName retrieves a service account by name.
func (s *iamService) GetServiceAccountByName(ctx context.Context, name string) (*models.ServiceAccount, error) {
	sa, err := s.serviceAccounts.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get service account by name: %w", err)
	}
	return sa, nil
}

// RevokeServiceAccount disables a service account and cleans up all associated resources.
//
// This is an out-of-band mutation operation that:
//  1. Sets disabled = true in the database
//  2. Revokes all active sessions for the service account
//  3. Removes all Casbin role assignments for the service account
//
// This follows the Phase 4 pattern: Casbin mutations happen in IAM service methods
// (out-of-band admin operations), NOT in the authentication/authorization request path.
func (s *iamService) RevokeServiceAccount(ctx context.Context, clientID string) error {
	// Step 1: Get service account by client ID
	sa, err := s.serviceAccounts.GetByClientID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("get service account: %w", err)
	}

	// Step 2: Disable the service account
	if err := s.serviceAccounts.SetDisabled(ctx, sa.ID, true); err != nil {
		return fmt.Errorf("disable service account: %w", err)
	}

	// Step 3: Revoke all active sessions for this service account
	if err := s.sessions.RevokeByServiceAccountID(ctx, sa.ID); err != nil {
		return fmt.Errorf("revoke service account sessions: %w", err)
	}

	// Step 4: Remove all Casbin role assignments
	// This is an out-of-band Casbin mutation (allowed in admin operations)
	casbinID := fmt.Sprintf("sa:%s", sa.ClientID)
	if _, err := s.enforcer.DeleteRolesForUser(casbinID); err != nil {
		return fmt.Errorf("delete Casbin roles for service account: %w", err)
	}

	return nil
}

// RotateServiceAccountSecret generates a new secret for a service account.
//
// Returns the unhashed secret (caller must save it) and the timestamp of rotation.
// The secret is hashed with bcrypt before storage.
func (s *iamService) RotateServiceAccountSecret(ctx context.Context, clientID string) (string, time.Time, error) {
	// Step 1: Get service account by client ID
	sa, err := s.serviceAccounts.GetByClientID(ctx, clientID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("get service account: %w", err)
	}

	// Step 2: Generate new secret
	newSecret, err := generateSessionToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate new secret: %w", err)
	}

	// Step 3: Hash new secret with bcrypt
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(newSecret), bcrypt.DefaultCost)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("hash new secret: %w", err)
	}

	// Step 4: Update secret hash in database
	if err := s.serviceAccounts.UpdateSecretHash(ctx, sa.ID, string(hashedSecret)); err != nil {
		return "", time.Time{}, fmt.Errorf("update secret hash: %w", err)
	}

	// Step 5: Get updated service account to retrieve rotation timestamp
	updatedSA, err := s.serviceAccounts.GetByID(ctx, sa.ID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("get updated service account: %w", err)
	}

	return newSecret, updatedSA.SecretRotatedAt, nil
}

// =========================================================================
// Role Assignment (Admin Operations - Triggers Cache Refresh)
// =========================================================================

// AssignRolesToServiceAccount assigns multiple roles to a service account.
func (s *iamService) AssignRolesToServiceAccount(ctx context.Context, serviceAccountID string, roleIDs []string) error {
	if serviceAccountID == "" {
		return fmt.Errorf("serviceAccountID is required")
	}

	for _, roleID := range roleIDs {
		if err := s.AssignUserRole(ctx, "", serviceAccountID, roleID); err != nil {
			return fmt.Errorf("assign role %s: %w", roleID, err)
		}
	}

	return nil
}

// RemoveRolesFromServiceAccount removes multiple roles from a service account.
func (s *iamService) RemoveRolesFromServiceAccount(ctx context.Context, serviceAccountID string, roleIDs []string) error {
	if serviceAccountID == "" {
		return fmt.Errorf("serviceAccountID is required")
	}

	for _, roleID := range roleIDs {
		if err := s.RemoveUserRole(ctx, "", serviceAccountID, roleID); err != nil {
			return fmt.Errorf("remove role %s: %w", roleID, err)
		}
	}

	return nil
}

// AssignUserRole assigns a role directly to a user or service account.
//
// This is an out-of-band mutation operation that:
//  1. Creates a UserRole record in the database
//  2. Syncs the assignment to Casbin for enforcement
//  3. Rolls back the database change if Casbin sync fails
//
// Parameters:
//   - userID: Internal UUID of user (set this for user principals, empty for SA)
//   - serviceAccountID: Internal UUID of service account (set this for SA principals, empty for user)
//   - roleID: Internal UUID of the role to assign
//
// This follows the Phase 4 pattern: Casbin mutations happen in IAM service methods
// (out-of-band admin operations), NOT in the authentication/authorization request path.
func (s *iamService) AssignUserRole(ctx context.Context, userID, serviceAccountID, roleID string) error {
	// Step 1: Validate that exactly one principal is specified
	if (userID == "" && serviceAccountID == "") || (userID != "" && serviceAccountID != "") {
		return fmt.Errorf("exactly one of userID or serviceAccountID must be specified")
	}

	// Step 2: Get role to verify it exists and get its name for Casbin
	role, err := s.roles.GetByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("get role: %w", err)
	}

	// Step 3: Create UserRole record
	var casbinPrincipalID string
	userRole := &models.UserRole{
		RoleID:     roleID,
		AssignedBy: auth.SystemUserID, // System-initiated assignment
	}

	if userID != "" {
		// User principal
		user, err := s.users.GetByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		userRole.UserID = &userID
		// Construct Casbin principal ID using the user's stable subject
		subjectID := user.PrincipalSubject()
		casbinPrincipalID = auth.UserID(subjectID)
	} else {
		// Service account principal
		sa, err := s.serviceAccounts.GetByID(ctx, serviceAccountID)
		if err != nil {
			return fmt.Errorf("get service account: %w", err)
		}
		userRole.ServiceAccountID = &serviceAccountID
		// Construct Casbin principal ID using the SA's client ID
		casbinPrincipalID = auth.ServiceAccountID(sa.ClientID)
	}

	// Step 4: Persist the assignment to database
	if err := s.userRoles.Create(ctx, userRole); err != nil {
		// Handle duplicate assignment gracefully
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return fmt.Errorf("role already assigned to principal")
		}
		return fmt.Errorf("create user role assignment: %w", err)
	}

	// Step 5: Sync to Casbin (out-of-band mutation)
	casbinRoleID := auth.RoleID(role.Name)
	if _, err := s.enforcer.AddRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
		// Rollback database change if Casbin sync fails
		_ = s.userRoles.Delete(ctx, userRole.ID)
		return fmt.Errorf("add Casbin role assignment: %w", err)
	}

	return nil
}

// RemoveUserRole removes a role from a user or service account.
//
// This is an out-of-band mutation operation that:
//  1. Deletes the UserRole record from the database
//  2. Removes the role assignment from Casbin
//
// Parameters:
//   - userID: Internal UUID of user (set this for user principals, empty for SA)
//   - serviceAccountID: Internal UUID of service account (set this for SA principals, empty for user)
//   - roleID: Internal UUID of the role to remove
//
// This follows the Phase 4 pattern: Casbin mutations happen in IAM service methods
// (out-of-band admin operations), NOT in the authentication/authorization request path.
func (s *iamService) RemoveUserRole(ctx context.Context, userID, serviceAccountID, roleID string) error {
	// Step 1: Validate that exactly one principal is specified
	if (userID == "" && serviceAccountID == "") || (userID != "" && serviceAccountID != "") {
		return fmt.Errorf("exactly one of userID or serviceAccountID must be specified")
	}

	// Step 2: Get role to verify it exists and get its name for Casbin
	role, err := s.roles.GetByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("get role: %w", err)
	}

	// Step 3: Determine principal type and construct Casbin ID
	var casbinPrincipalID string
	if userID != "" {
		// User principal
		user, err := s.users.GetByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		// Construct Casbin principal ID using the user's stable subject
		subjectID := user.PrincipalSubject()
		casbinPrincipalID = auth.UserID(subjectID)
	} else {
		// Service account principal
		sa, err := s.serviceAccounts.GetByID(ctx, serviceAccountID)
		if err != nil {
			return fmt.Errorf("get service account: %w", err)
		}
		// Construct Casbin principal ID using the SA's client ID
		casbinPrincipalID = auth.ServiceAccountID(sa.ClientID)
	}

	// Step 4: Delete from database
	if userID != "" {
		if err := s.userRoles.DeleteByUserAndRole(ctx, userID, roleID); err != nil {
			return fmt.Errorf("delete user role assignment: %w", err)
		}
	} else {
		if err := s.userRoles.DeleteByServiceAccountAndRole(ctx, serviceAccountID, roleID); err != nil {
			return fmt.Errorf("delete service account role assignment: %w", err)
		}
	}

	// Step 5: Remove from Casbin (out-of-band mutation)
	casbinRoleID := auth.RoleID(role.Name)
	if _, err := s.enforcer.DeleteRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
		return fmt.Errorf("remove Casbin role assignment: %w", err)
	}

	return nil
}

// AssignGroupRole assigns a role to an IdP group.
//
// This is an out-of-band mutation operation that:
//  1. Creates a GroupRole record in the database
//  2. Syncs the assignment to Casbin for enforcement
//  3. Automatically refreshes the group→role cache
//  4. Rolls back the database change if Casbin sync fails
//
// The automatic cache refresh ensures new mappings are visible to the
// authentication flow immediately (Phase 7 Task 7.3).
//
// This follows the Phase 4 pattern: Casbin mutations happen in IAM service methods
// (out-of-band admin operations), NOT in the authentication/authorization request path.
func (s *iamService) AssignGroupRole(ctx context.Context, groupName, roleID string) error {
	// Step 1: Get role to verify it exists and get its name for Casbin
	role, err := s.roles.GetByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("get role: %w", err)
	}

	// Step 2: Create GroupRole record
	// Use SystemUserID for CLI/system operations (until we add assignedBy parameter)
	groupRole := &models.GroupRole{
		GroupName:  groupName,
		RoleID:     roleID,
		AssignedBy: auth.SystemUserID,
	}

	if err := s.groupRoles.Create(ctx, groupRole); err != nil {
		// Handle duplicate assignment gracefully
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return fmt.Errorf("role already assigned to group")
		}
		return fmt.Errorf("create group role assignment: %w", err)
	}

	// Step 3: Sync to Casbin (out-of-band mutation)
	casbinPrincipalID := auth.GroupID(groupName)
	casbinRoleID := auth.RoleID(role.Name)

	if _, err := s.enforcer.AddRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
		// Rollback database change if Casbin sync fails
		_ = s.groupRoles.Delete(ctx, groupRole.ID)
		return fmt.Errorf("add Casbin group-role assignment: %w", err)
	}

	// Step 4: Automatically refresh cache (Phase 7 Task 7.3)
	// This ensures new group→role mappings are visible immediately
	if err := s.RefreshGroupRoleCache(ctx); err != nil {
		// Log error but don't fail - cache will refresh on background ticker
		// In production, this could be made async or non-fatal
		return fmt.Errorf("refresh group role cache: %w", err)
	}

	return nil
}

// RemoveGroupRole removes a role from a group.
//
// This is an out-of-band mutation operation that:
//  1. Deletes the GroupRole record from the database
//  2. Removes the role assignment from Casbin
//  3. Automatically refreshes the group→role cache
//
// The automatic cache refresh ensures removed mappings are no longer visible
// to the authentication flow immediately (Phase 7 Task 7.3).
//
// This follows the Phase 4 pattern: Casbin mutations happen in IAM service methods
// (out-of-band admin operations), NOT in the authentication/authorization request path.
func (s *iamService) RemoveGroupRole(ctx context.Context, groupName, roleID string) error {
	// Step 1: Get role to verify it exists and get its name for Casbin
	role, err := s.roles.GetByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("get role: %w", err)
	}

	// Step 2: Delete from database
	if err := s.groupRoles.DeleteByGroupAndRole(ctx, groupName, roleID); err != nil {
		return fmt.Errorf("delete group role assignment: %w", err)
	}

	// Step 3: Remove from Casbin (out-of-band mutation)
	casbinPrincipalID := auth.GroupID(groupName)
	casbinRoleID := auth.RoleID(role.Name)

	if _, err := s.enforcer.DeleteRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
		return fmt.Errorf("remove Casbin group-role assignment: %w", err)
	}

	// Step 4: Automatically refresh cache (Phase 7 Task 7.3)
	// This ensures removed group→role mappings are no longer visible immediately
	if err := s.RefreshGroupRoleCache(ctx); err != nil {
		// Log error but don't fail - cache will refresh on background ticker
		return fmt.Errorf("refresh group role cache: %w", err)
	}

	return nil
}

// =========================================================================
// Role Management (Admin Operations - CRUD for Roles)
// =========================================================================

// CreateRole creates a new role with permissions synced to Casbin.
//
// This is an out-of-band mutation operation that:
//  1. Validates label_scope_expr as valid go-bexpr syntax
//  2. Creates a Role record in the database
//  3. Parses actions (format "obj:act") and adds Casbin policies
//  4. Rolls back the database change if Casbin sync fails
//
// This follows the Phase 4 pattern: Casbin mutations happen in IAM service methods
// (out-of-band admin operations), NOT in the authentication/authorization request path.
func (s *iamService) CreateRole(
	ctx context.Context,
	name, description, scopeExpr string,
	createConstraints models.CreateConstraints,
	immutableKeys []string,
	actions []string,
) (*models.Role, error) {
	// Step 1: Validate scope expression as valid go-bexpr syntax
	if scopeExpr != "" {
		if _, err := bexpr.CreateEvaluator(scopeExpr); err != nil {
			return nil, fmt.Errorf("invalid label_scope_expr: %w", err)
		}
	}

	// Step 2: Create role record
	role := &models.Role{
		Name:              name,
		Description:       description,
		ScopeExpr:         scopeExpr,
		CreateConstraints: createConstraints,
		ImmutableKeys:     immutableKeys,
		Version:           1, // Initial version
	}

	if err := s.roles.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}

	// Step 3: Add Casbin policies for each action
	// Construct roleID for Casbin: "role:roleName"
	casbinRoleID := auth.RoleID(role.Name)

	for _, action := range actions {
		// Parse action format "obj:act" (e.g., "state:read")
		parts := strings.SplitN(action, ":", 2)
		if len(parts) != 2 {
			// Skip invalid actions with warning (don't fail entire request)
			// TODO: Add proper logging
			continue
		}
		objType := parts[0]
		act := parts[1]

		// Construct Casbin policy: [role, obj, action, scopeExpr, "allow"]
		policy := []string{casbinRoleID, objType, act, scopeExpr, "allow"}

		if _, err := s.enforcer.AddPolicy(policy); err != nil {
			// Rollback database change if Casbin sync fails
			_ = s.roles.Delete(ctx, role.ID)
			return nil, fmt.Errorf("add Casbin policy for action '%s': %w", action, err)
		}
	}

	return role, nil
}

// UpdateRole updates an existing role's permissions and metadata.
//
// This is an out-of-band mutation operation that:
//  1. Validates label_scope_expr as valid go-bexpr syntax
//  2. Checks optimistic locking (version must match)
//  3. Updates the Role record in the database
//  4. Removes all old Casbin policies for the role
//  5. Adds new Casbin policies based on updated actions
//
// Note: Unlike CreateRole, there is NO automatic rollback if Casbin sync fails after
// the database update. This is because the update has already been committed.
// In production, consider using a transactional Casbin adapter or manual reconciliation.
func (s *iamService) UpdateRole(
	ctx context.Context,
	name string,
	expectedVersion int,
	description, scopeExpr string,
	createConstraints models.CreateConstraints,
	immutableKeys []string,
	actions []string,
) (*models.Role, error) {
	// Step 1: Validate scope expression
	if scopeExpr != "" {
		if _, err := bexpr.CreateEvaluator(scopeExpr); err != nil {
			return nil, fmt.Errorf("invalid label_scope_expr: %w", err)
		}
	}

	// Step 2: Get existing role by name
	role, err := s.roles.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get role: %w", err)
	}

	// Step 3: Check optimistic locking
	if role.Version != expectedVersion {
		return nil, fmt.Errorf("version mismatch: expected %d, got %d (concurrent modification detected)", expectedVersion, role.Version)
	}

	// Step 4: Update role fields
	role.Description = description
	role.ScopeExpr = scopeExpr
	role.CreateConstraints = createConstraints
	role.ImmutableKeys = immutableKeys
	// Version is incremented by repository

	if err := s.roles.Update(ctx, role); err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}

	// Step 5: Sync Casbin policies
	// Remove all old policies for this role
	casbinRoleID := auth.RoleID(role.Name)
	if _, err := s.enforcer.RemoveFilteredPolicy(0, casbinRoleID); err != nil {
		// Log error but don't fail - update is already committed
		// TODO: Add proper logging
		return nil, fmt.Errorf("remove old Casbin policies: %w", err)
	}

	// Add new policies
	for _, action := range actions {
		parts := strings.SplitN(action, ":", 2)
		if len(parts) != 2 {
			continue
		}
		objType := parts[0]
		act := parts[1]

		policy := []string{casbinRoleID, objType, act, scopeExpr, "allow"}
		if _, err := s.enforcer.AddPolicy(policy); err != nil {
			// Log error but don't fail - can't rollback database update
			// TODO: Add proper logging for manual reconciliation
			return nil, fmt.Errorf("add Casbin policy for action '%s': %w", action, err)
		}
	}

	// Step 6: Fetch updated role (with incremented version)
	updatedRole, err := s.roles.GetByID(ctx, role.ID)
	if err != nil {
		return nil, fmt.Errorf("get updated role: %w", err)
	}

	return updatedRole, nil
}

// DeleteRole deletes a role and removes all associated Casbin policies.
//
// This is an out-of-band mutation operation that:
//  1. Verifies the role exists
//  2. Checks if the role is assigned to any principals (safety check)
//  3. Deletes the Role record from the database
//  4. Removes all Casbin policies for the role
//
// Safety: Rejects deletion if role is assigned to any principals.
func (s *iamService) DeleteRole(ctx context.Context, name string) error {
	// Step 1: Get role by name
	role, err := s.roles.GetByName(ctx, name)
	if err != nil {
		return fmt.Errorf("get role: %w", err)
	}

	// Step 2: Check if role is assigned to any principals (safety check)
	casbinRoleID := auth.RoleID(role.Name)
	users, err := s.enforcer.GetUsersForRole(casbinRoleID)
	if err != nil {
		return fmt.Errorf("check role assignments: %w", err)
	}

	if len(users) > 0 {
		return fmt.Errorf("cannot delete role: still assigned to %d principals", len(users))
	}

	// Step 3: Delete role from database
	if err := s.roles.Delete(ctx, role.ID); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}

	// Step 4: Remove all Casbin policies for this role
	if _, err := s.enforcer.RemoveFilteredPolicy(0, casbinRoleID); err != nil {
		// Log error prominently - role is already deleted from DB
		// TODO: Add proper logging for manual reconciliation
		return fmt.Errorf("remove Casbin policies: %w", err)
	}

	return nil
}

// =========================================================================
// Read-Only Lookup Methods (For Handlers - No Mutations)
// =========================================================================

// GetRoleByName retrieves a role by its name.
func (s *iamService) GetRoleByName(ctx context.Context, name string) (*models.Role, error) {
	return s.roles.GetByName(ctx, name)
}

// GetRolesByName returns the roles matching the provided names.
// It also reports which requested names were invalid and the complete set of valid role names.
func (s *iamService) GetRolesByName(ctx context.Context, roleNames []string) ([]models.Role, []string, []string, error) {
	allRoles, err := s.roles.List(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list roles: %w", err)
	}

	roleMap := make(map[string]models.Role, len(allRoles))
	validRoleNames := make([]string, 0, len(allRoles))
	for _, role := range allRoles {
		roleCopy := role
		roleMap[roleCopy.Name] = roleCopy
		validRoleNames = append(validRoleNames, roleCopy.Name)
	}

	matched := make([]models.Role, 0, len(roleNames))
	invalid := make([]string, 0)

	for _, requestedName := range roleNames {
		if role, ok := roleMap[requestedName]; ok {
			matched = append(matched, role)
			continue
		}
		invalid = append(invalid, requestedName)
	}

	return matched, invalid, validRoleNames, nil
}

// GetRoleByID retrieves a role by its internal ID.
func (s *iamService) GetRoleByID(ctx context.Context, roleID string) (*models.Role, error) {
	return s.roles.GetByID(ctx, roleID)
}

// ListAllRoles returns all roles in the system.
func (s *iamService) ListAllRoles(ctx context.Context) ([]models.Role, error) {
	return s.roles.List(ctx)
}

// GetServiceAccountByID retrieves a service account by its internal ID.
func (s *iamService) GetServiceAccountByID(ctx context.Context, saID string) (*models.ServiceAccount, error) {
	return s.serviceAccounts.GetByID(ctx, saID)
}

// ListGroupRoles returns all group→role assignments, optionally filtered by group name.
func (s *iamService) ListGroupRoles(ctx context.Context, groupName *string) ([]models.GroupRole, error) {
	if groupName != nil {
		return s.groupRoles.GetByGroupName(ctx, *groupName)
	}
	return s.groupRoles.List(ctx)
}

// GetPrincipalRoles returns the Casbin role IDs for a principal.
// This replaces direct Enforcer.GetRolesForUser() calls in handlers.
func (s *iamService) GetPrincipalRoles(ctx context.Context, principalID, principalType string) ([]string, error) {
	var casbinID string
	switch principalType {
	case "user":
		user, err := s.users.GetByID(ctx, principalID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		casbinID = auth.UserID(user.PrincipalSubject())
	case "service_account":
		sa, err := s.serviceAccounts.GetByID(ctx, principalID)
		if err != nil {
			return nil, fmt.Errorf("get service account: %w", err)
		}
		casbinID = auth.ServiceAccountID(sa.ClientID)
	default:
		return nil, fmt.Errorf("invalid principal type: %s", principalType)
	}

	roles, err := s.enforcer.GetRolesForUser(casbinID)
	if err != nil {
		return nil, fmt.Errorf("get roles from casbin: %w", err)
	}

	return roles, nil
}

// GetRolePermissions returns the Casbin permissions for a role.
// This replaces direct Enforcer.GetPermissionsForUser() calls in handlers.
func (s *iamService) GetRolePermissions(ctx context.Context, roleName string) ([][]string, error) {
	casbinRoleID := auth.RoleID(roleName)
	permissions, err := s.enforcer.GetPermissionsForUser(casbinRoleID)
	if err != nil {
		return nil, fmt.Errorf("get permissions from casbin: %w", err)
	}
	return permissions, nil
}

// =========================================================================
// Helper Functions
// =========================================================================

// generateSessionToken generates a cryptographically secure random token.
//
// Returns a 64-character hex string (32 bytes of entropy).
func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashToken hashes a token with SHA256 for storage.
//
// The unhashed token is given to the user (as cookie or bearer token).
// The hash is stored in the database for validation.
func hashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}
