package repository

import (
	"context"

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

	// Cycle detection (application-layer pre-check, DB trigger is safety net)
	WouldCreateCycle(ctx context.Context, fromState, toState string) (bool, error)
}

// OutputKey represents a Terraform output name and metadata.
type OutputKey struct {
	Key       string
	Sensitive bool
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
