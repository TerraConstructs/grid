package repository

import (
	"context"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// StateRepository exposes persistence operations for Terraform states.
type StateRepository interface {
	Create(ctx context.Context, state *models.State) error
	GetByGUID(ctx context.Context, guid string) (*models.State, error)
	GetByLogicID(ctx context.Context, logicID string) (*models.State, error)
	Update(ctx context.Context, state *models.State) error
	List(ctx context.Context) ([]models.State, error)
	Lock(ctx context.Context, guid string, lockInfo *models.LockInfo) error
	Unlock(ctx context.Context, guid string, lockID string) error

	// UpdateContentAndUpsertOutputs atomically updates state content and output cache in one transaction.
	// This ensures FR-027 compliance: cache and state are always consistent.
	UpdateContentAndUpsertOutputs(ctx context.Context, guid string, content []byte, lockID string, serial int64, outputs []OutputKey) error

	// ListWithFilter returns states matching bexpr filter with pagination.
	// T029: Added for label filtering support.
	ListWithFilter(ctx context.Context, filter string, pageSize int, offset int) ([]models.State, error)

	// GetByGUIDs fetches multiple states by GUIDs in a single query (batch operation).
	// Returns a map of GUID -> State for efficient lookup. Missing GUIDs are omitted from result.
	GetByGUIDs(ctx context.Context, guids []string) (map[string]*models.State, error)

	// GetByGUIDWithRelations fetches a state with specified relations preloaded.
	// Relations can be: "Outputs", "IncomingEdges", "OutgoingEdges"
	// Example: GetByGUIDWithRelations(ctx, guid, "Outputs", "IncomingEdges")
	GetByGUIDWithRelations(ctx context.Context, guid string, relations ...string) (*models.State, error)

	// ListStatesWithOutputs returns all states with their outputs preloaded (avoids N+1).
	ListStatesWithOutputs(ctx context.Context) ([]*models.State, error)
}

// EdgeRepository exposes persistence operations for dependency edges.
type EdgeRepository interface {
	// CRUD operations
	Create(ctx context.Context, edge *models.Edge) error
	GetByID(ctx context.Context, id int64) (*models.Edge, error)
	Delete(ctx context.Context, id int64) error
	Update(ctx context.Context, edge *models.Edge) error

	// Query operations
	GetOutgoingEdges(ctx context.Context, fromStateGUID string) ([]models.Edge, error)
	GetIncomingEdges(ctx context.Context, toStateGUID string) ([]models.Edge, error)
	GetAllEdges(ctx context.Context) ([]models.Edge, error)
	FindByOutput(ctx context.Context, outputKey string) ([]models.Edge, error)

	// Eager loading operations (avoid N+1 queries)
	// GetIncomingEdgesWithProducers fetches incoming edges with producer state data preloaded.
	// The FromStateRel field will be populated for each edge.
	GetIncomingEdgesWithProducers(ctx context.Context, toStateGUID string) ([]*models.Edge, error)

	// GetOutgoingEdgesWithConsumers fetches outgoing edges with consumer state data preloaded.
	// The ToStateRel field will be populated for each edge.
	GetOutgoingEdgesWithConsumers(ctx context.Context, fromStateGUID string) ([]*models.Edge, error)

	// Cycle detection (application-layer pre-check, DB trigger is safety net)
	WouldCreateCycle(ctx context.Context, fromState, toState string) (bool, error)
}

// OutputKey represents a Terraform output name and metadata.
type OutputKey struct {
	Key       string
	Sensitive bool
}

// ========================================
// Auth Repositories
// ========================================

// UserRepository exposes persistence operations for users
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetBySubject(ctx context.Context, subject string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateLastLogin(ctx context.Context, id string) error
	SetPasswordHash(ctx context.Context, id string, passwordHash string) error
	List(ctx context.Context) ([]models.User, error)
}

// ServiceAccountRepository exposes persistence operations for service accounts
type ServiceAccountRepository interface {
	Create(ctx context.Context, sa *models.ServiceAccount) error
	GetByID(ctx context.Context, id string) (*models.ServiceAccount, error)
	GetByName(ctx context.Context, name string) (*models.ServiceAccount, error)
	GetByClientID(ctx context.Context, clientID string) (*models.ServiceAccount, error)
	Update(ctx context.Context, sa *models.ServiceAccount) error
	UpdateLastUsed(ctx context.Context, id string) error
	UpdateSecretHash(ctx context.Context, id string, secretHash string) error
	SetDisabled(ctx context.Context, id string, disabled bool) error
	List(ctx context.Context) ([]models.ServiceAccount, error)
	ListByCreator(ctx context.Context, createdBy string) ([]models.ServiceAccount, error)
}

// RoleRepository exposes persistence operations for roles
type RoleRepository interface {
	Create(ctx context.Context, role *models.Role) error
	GetByID(ctx context.Context, id string) (*models.Role, error)
	GetByName(ctx context.Context, name string) (*models.Role, error)
	Update(ctx context.Context, role *models.Role) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]models.Role, error)
}

// UserRoleRepository exposes persistence operations for user-role assignments
type UserRoleRepository interface {
	Create(ctx context.Context, ur *models.UserRole) error
	GetByID(ctx context.Context, id string) (*models.UserRole, error)
	GetByUserID(ctx context.Context, userID string) ([]models.UserRole, error)
	GetByUserAndRoleID(ctx context.Context, userID string, roleID string) (*models.UserRole, error)
	GetByServiceAccountID(ctx context.Context, serviceAccountID string) ([]models.UserRole, error)
	GetByServiceAccountAndRoleID(ctx context.Context, serviceAccountID string, roleID string) (*models.UserRole, error)
	GetByRoleID(ctx context.Context, roleID string) ([]models.UserRole, error)
	Delete(ctx context.Context, id string) error
	DeleteByUserAndRole(ctx context.Context, userID string, roleID string) error
	DeleteByServiceAccountAndRole(ctx context.Context, serviceAccountID string, roleID string) error
	List(ctx context.Context) ([]models.UserRole, error)
}

// GroupRoleRepository exposes persistence operations for group-role mappings
type GroupRoleRepository interface {
	Create(ctx context.Context, gr *models.GroupRole) error
	GetByID(ctx context.Context, id string) (*models.GroupRole, error)
	GetByGroupName(ctx context.Context, groupName string) ([]models.GroupRole, error)
	GetByRoleID(ctx context.Context, roleID string) ([]models.GroupRole, error)
	Delete(ctx context.Context, id string) error
	DeleteByGroupAndRole(ctx context.Context, groupName string, roleID string) error
	List(ctx context.Context) ([]models.GroupRole, error)
}

// SessionRepository exposes persistence operations for sessions
type SessionRepository interface {
	Create(ctx context.Context, session *models.Session) error
	GetByID(ctx context.Context, id string) (*models.Session, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error)
	GetByUserID(ctx context.Context, userID string) ([]models.Session, error)
	GetByServiceAccountID(ctx context.Context, serviceAccountID string) ([]models.Session, error)
	UpdateLastUsed(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string) error
	RevokeByUserID(ctx context.Context, userID string) error
	RevokeByServiceAccountID(ctx context.Context, serviceAccountID string) error
	DeleteExpired(ctx context.Context) error
	List(ctx context.Context) ([]models.Session, error)
}

// RevokedJTIRepository exposes persistence operations for revoked JWT IDs
type RevokedJTIRepository interface {
	// Create adds a JTI to the revocation denylist
	Create(ctx context.Context, revokedJTI *models.RevokedJTI) error

	// IsRevoked checks if a JTI exists in the revocation table
	IsRevoked(ctx context.Context, jti string) (bool, error)

	// DeleteExpired removes revoked JTIs where exp < now() - grace period
	// Used for periodic cleanup to prevent table bloat
	DeleteExpired(ctx context.Context, gracePeriod time.Duration) error

	// GetByJTI retrieves a revoked JTI entry by its ID
	GetByJTI(ctx context.Context, jti string) (*models.RevokedJTI, error)
}

// StateOutputRef represents a state reference with an output key.
type StateOutputRef struct {
	StateGUID    string
	StateLogicID string
	OutputKey    string
	Sensitive    bool
}

// StateOutputRepository exposes persistence operations for cached Terraform outputs.
type StateOutputRepository interface {
	// UpsertOutputs atomically replaces all outputs for a state
	// Deletes old outputs where state_serial != serial, inserts new ones
	UpsertOutputs(ctx context.Context, stateGUID string, serial int64, outputs []OutputKey) error

	// GetOutputsByState returns all cached outputs for a state
	// Returns empty slice if no outputs exist (not an error)
	GetOutputsByState(ctx context.Context, stateGUID string) ([]OutputKey, error)

	// SearchOutputsByKey finds all states with output matching key (exact match)
	// Used for cross-state dependency discovery
	SearchOutputsByKey(ctx context.Context, outputKey string) ([]StateOutputRef, error)

	// DeleteOutputsByState removes all cached outputs for a state
	// Cascade handles this on state deletion, but explicit method useful for testing
	DeleteOutputsByState(ctx context.Context, stateGUID string) error
}

// LabelPolicyRepository exposes persistence operations for label validation policy.
// T030: Added for label policy management.
type LabelPolicyRepository interface {
	// GetPolicy retrieves the current policy (single-row table with id=1)
	GetPolicy(ctx context.Context) (*models.LabelPolicy, error)

	// SetPolicy creates or updates the policy with version increment
	SetPolicy(ctx context.Context, policy *models.PolicyDefinition) error
}
