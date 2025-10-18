package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// BunServiceAccountRepository implements ServiceAccountRepository using Bun ORM
type BunServiceAccountRepository struct {
	db *bun.DB
}

// NewBunServiceAccountRepository creates a new Bun-based service account repository
func NewBunServiceAccountRepository(db *bun.DB) *BunServiceAccountRepository {
	return &BunServiceAccountRepository{db: db}
}

// Create inserts a new service account
func (r *BunServiceAccountRepository) Create(ctx context.Context, sa *models.ServiceAccount) error {
	_, err := r.db.NewInsert().
		Model(sa).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create service account: %w", err)
	}
	return nil
}

// GetByID retrieves a service account by ID
func (r *BunServiceAccountRepository) GetByID(ctx context.Context, id string) (*models.ServiceAccount, error) {
	sa := new(models.ServiceAccount)
	err := r.db.NewSelect().
		Model(sa).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("service account not found: %s", id)
		}
		return nil, fmt.Errorf("get service account: %w", err)
	}
	return sa, nil
}

// GetByClientID retrieves a service account by client ID
func (r *BunServiceAccountRepository) GetByClientID(ctx context.Context, clientID string) (*models.ServiceAccount, error) {
	sa := new(models.ServiceAccount)
	err := r.db.NewSelect().
		Model(sa).
		Where("client_id = ?", clientID).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("service account not found with client_id: %s", clientID)
		}
		return nil, fmt.Errorf("get service account by client_id: %w", err)
	}
	return sa, nil
}

// Update updates an existing service account
func (r *BunServiceAccountRepository) Update(ctx context.Context, sa *models.ServiceAccount) error {
	result, err := r.db.NewUpdate().
		Model(sa).
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update service account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("service account not found: %s", sa.ID)
	}

	return nil
}

// UpdateLastUsed updates the last_used_at timestamp
func (r *BunServiceAccountRepository) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*models.ServiceAccount)(nil)).
		Set("last_used_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update last used: %w", err)
	}
	return nil
}

// UpdateSecretHash updates the client secret hash (for rotation)
func (r *BunServiceAccountRepository) UpdateSecretHash(ctx context.Context, id string, secretHash string) error {
	_, err := r.db.NewUpdate().
		Model((*models.ServiceAccount)(nil)).
		Set("client_secret_hash = ?", secretHash).
		Set("secret_rotated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update secret hash: %w", err)
	}
	return nil
}

// List retrieves all service accounts
func (r *BunServiceAccountRepository) List(ctx context.Context) ([]models.ServiceAccount, error) {
	var accounts []models.ServiceAccount
	err := r.db.NewSelect().
		Model(&accounts).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list service accounts: %w", err)
	}
	return accounts, nil
}

// ListByCreator retrieves service accounts created by a specific user
func (r *BunServiceAccountRepository) ListByCreator(ctx context.Context, createdBy string) ([]models.ServiceAccount, error) {
	var accounts []models.ServiceAccount
	err := r.db.NewSelect().
		Model(&accounts).
		Where("created_by = ?", createdBy).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list service accounts by creator: %w", err)
	}
	return accounts, nil
}

// SetDisabled updates the disabled status of a service account
func (r *BunServiceAccountRepository) SetDisabled(ctx context.Context, id string, disabled bool) error {
	_, err := r.db.NewUpdate().
		Model((*models.ServiceAccount)(nil)).
		Set("disabled = ?", disabled).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("set disabled status for service account: %w", err)
	}
	return nil
}
