package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// BunSessionRepository implements SessionRepository using Bun ORM
type BunSessionRepository struct {
	db *bun.DB
}

// NewBunSessionRepository creates a new Bun-based session repository
func NewBunSessionRepository(db *bun.DB) *BunSessionRepository {
	return &BunSessionRepository{db: db}
}

// Create inserts a new session
func (r *BunSessionRepository) Create(ctx context.Context, session *models.Session) error {
	_, err := r.db.NewInsert().
		Model(session).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// GetByID retrieves a session by ID
func (r *BunSessionRepository) GetByID(ctx context.Context, id string) (*models.Session, error) {
	session := new(models.Session)
	err := r.db.NewSelect().
		Model(session).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	return session, nil
}

// GetByTokenHash retrieves a session by its token hash
// This is the primary lookup method for authentication
func (r *BunSessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error) {
	session := new(models.Session)
	err := r.db.NewSelect().
		Model(session).
		Where("token_hash = ?", tokenHash).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("get session by token: %w", err)
	}
	return session, nil
}

// GetByUserID retrieves all sessions for a user
func (r *BunSessionRepository) GetByUserID(ctx context.Context, userID string) ([]models.Session, error) {
	var sessions []models.Session
	err := r.db.NewSelect().
		Model(&sessions).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user sessions: %w", err)
	}
	return sessions, nil
}

// GetByServiceAccountID retrieves all sessions for a service account
func (r *BunSessionRepository) GetByServiceAccountID(ctx context.Context, serviceAccountID string) ([]models.Session, error) {
	var sessions []models.Session
	err := r.db.NewSelect().
		Model(&sessions).
		Where("service_account_id = ?", serviceAccountID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get service account sessions: %w", err)
	}
	return sessions, nil
}

// UpdateLastUsed updates the last_used_at timestamp for a session
func (r *BunSessionRepository) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*models.Session)(nil)).
		Set("last_used_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update last used: %w", err)
	}
	return nil
}

// Revoke marks a session as revoked
func (r *BunSessionRepository) Revoke(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*models.Session)(nil)).
		Set("revoked = ?", true).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// RevokeByUserID revokes all sessions for a user
// Used for manual logout or security incidents
func (r *BunSessionRepository) RevokeByUserID(ctx context.Context, userID string) error {
	_, err := r.db.NewUpdate().
		Model((*models.Session)(nil)).
		Set("revoked = ?", true).
		Where("user_id = ?", userID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

// RevokeByServiceAccountID revokes all sessions for a service account
// Used for FR-070b (cascade revocation when service account is disabled/deleted)
func (r *BunSessionRepository) RevokeByServiceAccountID(ctx context.Context, serviceAccountID string) error {
	_, err := r.db.NewUpdate().
		Model((*models.Session)(nil)).
		Set("revoked = ?", true).
		Where("service_account_id = ?", serviceAccountID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("revoke service account sessions: %w", err)
	}
	return nil
}

// DeleteExpired deletes all expired sessions
// Should be run periodically by a cleanup job
func (r *BunSessionRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.NewDelete().
		Model((*models.Session)(nil)).
		Where("expires_at < ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

// List retrieves all sessions (admin operation)
func (r *BunSessionRepository) List(ctx context.Context) ([]models.Session, error) {
	var sessions []models.Session
	err := r.db.NewSelect().
		Model(&sessions).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return sessions, nil
}
