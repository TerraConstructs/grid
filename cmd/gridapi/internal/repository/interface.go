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
