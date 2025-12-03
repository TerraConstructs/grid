package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// ========================================
// Role Repository
// ========================================

// BunRoleRepository implements RoleRepository using Bun ORM
type BunRoleRepository struct {
	db *bun.DB
}

// NewBunRoleRepository creates a new Bun-based role repository
func NewBunRoleRepository(db *bun.DB) RoleRepository {
	return &BunRoleRepository{db: db}
}

// Create inserts a new role
func (r *BunRoleRepository) Create(ctx context.Context, role *models.Role) error {
	if role.ID == "" {
		role.ID = bunx.NewUUIDv7()
	}

	_, err := r.db.NewInsert().
		Model(role).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create role: %w", err)
	}
	return nil
}

// GetByID retrieves a role by ID
func (r *BunRoleRepository) GetByID(ctx context.Context, id string) (*models.Role, error) {
	role := new(models.Role)
	err := r.db.NewSelect().
		Model(role).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("role not found: %s", id)
		}
		return nil, fmt.Errorf("get role: %w", err)
	}
	return role, nil
}

// GetByName retrieves a role by name
func (r *BunRoleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	role := new(models.Role)
	err := r.db.NewSelect().
		Model(role).
		Where("name = ?", name).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("role not found: %s", name)
		}
		return nil, fmt.Errorf("get role by name: %w", err)
	}
	return role, nil
}

// Update updates an existing role
func (r *BunRoleRepository) Update(ctx context.Context, role *models.Role) error {
	role.UpdatedAt = time.Now()
	role.Version++ // Optimistic locking
	result, err := r.db.NewUpdate().
		Model(role).
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("role not found: %s", role.ID)
	}

	return nil
}

// Delete deletes a role by ID
func (r *BunRoleRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.NewDelete().
		Model((*models.Role)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("role not found: %s", id)
	}

	return nil
}

// List retrieves all roles
func (r *BunRoleRepository) List(ctx context.Context) ([]models.Role, error) {
	var roles []models.Role
	err := r.db.NewSelect().
		Model(&roles).
		Order("name ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return roles, nil
}

// ========================================
// UserRole Repository
// ========================================

// BunUserRoleRepository implements UserRoleRepository using Bun ORM
type BunUserRoleRepository struct {
	db *bun.DB
}

// NewBunUserRoleRepository creates a new Bun-based user role repository
func NewBunUserRoleRepository(db *bun.DB) UserRoleRepository {
	return &BunUserRoleRepository{db: db}
}

// Create inserts a new user-role assignment
func (r *BunUserRoleRepository) Create(ctx context.Context, ur *models.UserRole) error {
	if ur.ID == "" {
		ur.ID = bunx.NewUUIDv7()
	}

	// Validate that exactly one principal is specified (defensive check for SQLite compatibility)
	if (ur.UserID == nil && ur.ServiceAccountID == nil) || (ur.UserID != nil && ur.ServiceAccountID != nil) {
		return fmt.Errorf("exactly one of user_id or service_account_id must be set")
	}

	_, err := r.db.NewInsert().
		Model(ur).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create user role: %w", err)
	}
	return nil
}

// GetByID retrieves a user-role assignment by ID
func (r *BunUserRoleRepository) GetByID(ctx context.Context, id string) (*models.UserRole, error) {
	ur := new(models.UserRole)
	err := r.db.NewSelect().
		Model(ur).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user role not found: %s", id)
		}
		return nil, fmt.Errorf("get user role: %w", err)
	}
	return ur, nil
}

// GetByUserID retrieves all role assignments for a user
func (r *BunUserRoleRepository) GetByUserID(ctx context.Context, userID string) ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.db.NewSelect().
		Model(&userRoles).
		Where("user_id = ?", userID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}
	return userRoles, nil
}

// GetByUserAndRoleID retrieves all role assignments for a user and role
func (r *BunUserRoleRepository) GetByUserAndRoleID(ctx context.Context, userID, roleID string) (*models.UserRole, error) {
	userRole := new(models.UserRole)
	err := r.db.NewSelect().
		Model(userRole).
		Where("user_id = ? and role_id = ?", userID, roleID).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user role assignment not found: user_id=%s, role_id=%s", userID, roleID)
		}
		return nil, fmt.Errorf("get user roles: %w", err)
	}
	return userRole, nil
}

// GetByServiceAccountID retrieves all role assignments for a service account
func (r *BunUserRoleRepository) GetByServiceAccountID(ctx context.Context, serviceAccountID string) ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.db.NewSelect().
		Model(&userRoles).
		Where("service_account_id = ?", serviceAccountID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get service account roles: %w", err)
	}
	return userRoles, nil
}

// GetByServiceAccountAndRoleID retrieves all role assignments for a service account
func (r *BunUserRoleRepository) GetByServiceAccountAndRoleID(ctx context.Context, serviceAccountID string, roleID string) (*models.UserRole, error) {
	userRole := new(models.UserRole)
	err := r.db.NewSelect().
		Model(userRole).
		Where("service_account_id = ? AND role_id = ?", serviceAccountID, roleID).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("service account role assignment not found: service_account_id=%s, role_id=%s", serviceAccountID, roleID)
		}
		return nil, fmt.Errorf("get service account role: %w", err)
	}
	return userRole, nil
}

// GetByRoleID retrieves all assignments for a specific role
func (r *BunUserRoleRepository) GetByRoleID(ctx context.Context, roleID string) ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.db.NewSelect().
		Model(&userRoles).
		Where("role_id = ?", roleID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get role assignments: %w", err)
	}
	return userRoles, nil
}

// Delete deletes a user-role assignment by ID
func (r *BunUserRoleRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.NewDelete().
		Model((*models.UserRole)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete user role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user role not found: %s", id)
	}

	return nil
}

// DeleteByUserAndRole deletes a specific user-role assignment
func (r *BunUserRoleRepository) DeleteByUserAndRole(ctx context.Context, userID string, roleID string) error {
	_, err := r.db.NewDelete().
		Model((*models.UserRole)(nil)).
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete user role: %w", err)
	}
	return nil
}

// DeleteByServiceAccountAndRole deletes a specific service account-role assignment
func (r *BunUserRoleRepository) DeleteByServiceAccountAndRole(ctx context.Context, serviceAccountID string, roleID string) error {
	_, err := r.db.NewDelete().
		Model((*models.UserRole)(nil)).
		Where("service_account_id = ? AND role_id = ?", serviceAccountID, roleID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete service account role: %w", err)
	}
	return nil
}

// List retrieves all user-role assignments
func (r *BunUserRoleRepository) List(ctx context.Context) ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.db.NewSelect().
		Model(&userRoles).
		Order("assigned_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}
	return userRoles, nil
}

// ========================================
// GroupRole Repository
// ========================================

// BunGroupRoleRepository implements GroupRoleRepository using Bun ORM
type BunGroupRoleRepository struct {
	db *bun.DB
}

// NewBunGroupRoleRepository creates a new Bun-based group role repository
func NewBunGroupRoleRepository(db *bun.DB) GroupRoleRepository {
	return &BunGroupRoleRepository{db: db}
}

// Create inserts a new group-role mapping
func (r *BunGroupRoleRepository) Create(ctx context.Context, gr *models.GroupRole) error {
	if gr.ID == "" {
		gr.ID = bunx.NewUUIDv7()
	}

	_, err := r.db.NewInsert().
		Model(gr).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create group role: %w", err)
	}
	return nil
}

// GetByID retrieves a group-role mapping by ID
func (r *BunGroupRoleRepository) GetByID(ctx context.Context, id string) (*models.GroupRole, error) {
	gr := new(models.GroupRole)
	err := r.db.NewSelect().
		Model(gr).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("group role not found: %s", id)
		}
		return nil, fmt.Errorf("get group role: %w", err)
	}
	return gr, nil
}

// GetByGroupName retrieves all role mappings for a group
func (r *BunGroupRoleRepository) GetByGroupName(ctx context.Context, groupName string) ([]models.GroupRole, error) {
	var groupRoles []models.GroupRole
	err := r.db.NewSelect().
		Model(&groupRoles).
		Where("group_name = ?", groupName).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get group roles: %w", err)
	}
	return groupRoles, nil
}

// GetByRoleID retrieves all group mappings for a specific role
func (r *BunGroupRoleRepository) GetByRoleID(ctx context.Context, roleID string) ([]models.GroupRole, error) {
	var groupRoles []models.GroupRole
	err := r.db.NewSelect().
		Model(&groupRoles).
		Where("role_id = ?", roleID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get role groups: %w", err)
	}
	return groupRoles, nil
}

// Delete deletes a group-role mapping by ID
func (r *BunGroupRoleRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.NewDelete().
		Model((*models.GroupRole)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete group role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("group role not found: %s", id)
	}

	return nil
}

// DeleteByGroupAndRole deletes a specific group-role mapping
func (r *BunGroupRoleRepository) DeleteByGroupAndRole(ctx context.Context, groupName string, roleID string) error {
	_, err := r.db.NewDelete().
		Model((*models.GroupRole)(nil)).
		Where("group_name = ? AND role_id = ?", groupName, roleID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete group role: %w", err)
	}
	return nil
}

// List retrieves all group-role mappings
func (r *BunGroupRoleRepository) List(ctx context.Context) ([]models.GroupRole, error) {
	var groupRoles []models.GroupRole
	err := r.db.NewSelect().
		Model(&groupRoles).
		Order("assigned_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list group roles: %w", err)
	}
	return groupRoles, nil
}