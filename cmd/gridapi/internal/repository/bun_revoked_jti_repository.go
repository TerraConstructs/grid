package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// BunRevokedJTIRepository implements RevokedJTIRepository using Bun ORM
type BunRevokedJTIRepository struct {
	db *bun.DB
}

// NewBunRevokedJTIRepository creates a new Bun-based revoked JTI repository
func NewBunRevokedJTIRepository(db *bun.DB) RevokedJTIRepository {
	return &BunRevokedJTIRepository{db: db}
}

// Create adds a JTI to the revocation denylist
func (r *BunRevokedJTIRepository) Create(ctx context.Context, revokedJTI *models.RevokedJTI) error {
	_, err := r.db.NewInsert().
		Model(revokedJTI).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create revoked jti: %w", err)
	}
	return nil
}

// IsRevoked checks if a JTI exists in the revocation table
// Uses SELECT EXISTS pattern for efficient boolean check
func (r *BunRevokedJTIRepository) IsRevoked(ctx context.Context, jti string) (bool, error) {
	exists, err := r.db.NewSelect().
		Model((*models.RevokedJTI)(nil)).
		Where("jti = ?", jti).
		Exists(ctx)

	if err != nil {
		return false, fmt.Errorf("check revoked jti: %w", err)
	}

	return exists, nil
}

// DeleteExpired removes revoked JTIs where exp < now() - grace period
// Used for periodic cleanup to prevent table bloat
func (r *BunRevokedJTIRepository) DeleteExpired(ctx context.Context, gracePeriod time.Duration) error {
	cutoffTime := time.Now().Add(-gracePeriod)

	_, err := r.db.NewDelete().
		Model((*models.RevokedJTI)(nil)).
		Where("exp < ?", cutoffTime).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete expired revoked jtis: %w", err)
	}
	return nil
}

// GetByJTI retrieves a revoked JTI entry by its ID
func (r *BunRevokedJTIRepository) GetByJTI(ctx context.Context, jti string) (*models.RevokedJTI, error) {
	revokedJTI := new(models.RevokedJTI)
	err := r.db.NewSelect().
		Model(revokedJTI).
		Where("jti = ?", jti).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("revoked jti not found: %s", jti)
		}
		return nil, fmt.Errorf("get revoked jti: %w", err)
	}
	return revokedJTI, nil
}
