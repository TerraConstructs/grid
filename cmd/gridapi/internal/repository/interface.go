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
