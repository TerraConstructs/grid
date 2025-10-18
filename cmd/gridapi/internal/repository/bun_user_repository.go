package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// BunUserRepository implements UserRepository using Bun ORM
type BunUserRepository struct {
	db *bun.DB
}

// NewBunUserRepository creates a new Bun-based user repository
func NewBunUserRepository(db *bun.DB) *BunUserRepository {
	return &BunUserRepository{db: db}
}

// Create inserts a new user into the database
func (r *BunUserRepository) Create(ctx context.Context, user *models.User) error {
	_, err := r.db.NewInsert().
		Model(user).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by their ID
func (r *BunUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	user := new(models.User)
	err := r.db.NewSelect().
		Model(user).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("get user by ID: %w", err)
	}
	return user, nil
}

// GetBySubject retrieves a user by their OIDC subject
func (r *BunUserRepository) GetBySubject(ctx context.Context, subject string) (*models.User, error) {
	user := new(models.User)
	err := r.db.NewSelect().
		Model(user).
		Where("subject = ?", subject).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found with subject: %s", subject)
		}
		return nil, fmt.Errorf("get user by subject: %w", err)
	}
	return user, nil
}

// GetByEmail retrieves a user by their email
func (r *BunUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := new(models.User)
	err := r.db.NewSelect().
		Model(user).
		Where("email = ?", email).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found with email: %s", email)
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// Update updates an existing user
func (r *BunUserRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()
	result, err := r.db.NewUpdate().
		Model(user).
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", user.ID)
	}

	return nil
}

// UpdateLastLogin updates the last_login_at timestamp for a user
func (r *BunUserRepository) UpdateLastLogin(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*models.User)(nil)).
		Set("last_login_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

// SetPasswordHash updates the stored bcrypt hash for a user's local credentials.
func (r *BunUserRepository) SetPasswordHash(ctx context.Context, id string, passwordHash string) error {
	_, err := r.db.NewUpdate().
		Model((*models.User)(nil)).
		Set("password_hash = ?", passwordHash).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("set password hash: %w", err)
	}
	return nil
}

// List retrieves all users
func (r *BunUserRepository) List(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.NewSelect().
		Model(&users).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}
