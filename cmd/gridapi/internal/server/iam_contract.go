package server

import (
	"context"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
)

// iamAdminService defines the exact IAM methods used by server handlers.
// This interface provides compile-time proof that iam.Service satisfies
// all requirements without circular imports.
//
// By defining this contract in the server package, we avoid importing
// repositories or internal IAM implementation details while ensuring
// type safety at compile time.
type iamAdminService interface {
	// Authentication and authorization (used by auth handlers)
	ResolveRoles(ctx context.Context, principalID string, groups []string, isUser bool) ([]string, error)

	// Session management
	CreateSession(ctx context.Context, userID, idToken string, expiresAt time.Time) (*models.Session, string, error)
	GetSessionByID(ctx context.Context, sessionID string) (*models.Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	ListUserSessions(ctx context.Context, userID string) ([]models.Session, error)

	// Service account management
	CreateServiceAccount(ctx context.Context, name, createdBy string) (*models.ServiceAccount, string, error)
	ListServiceAccounts(ctx context.Context) ([]*models.ServiceAccount, error)
	GetServiceAccountByClientID(ctx context.Context, clientID string) (*models.ServiceAccount, error)
	GetServiceAccountByID(ctx context.Context, saID string) (*models.ServiceAccount, error)
	RevokeServiceAccount(ctx context.Context, clientID string) error
	RotateServiceAccountSecret(ctx context.Context, clientID string) (string, time.Time, error)

	// Role assignment
	AssignUserRole(ctx context.Context, userID, serviceAccountID, roleID string) error
	RemoveUserRole(ctx context.Context, userID, serviceAccountID, roleID string) error
	AssignGroupRole(ctx context.Context, groupName, roleID string) error
	RemoveGroupRole(ctx context.Context, groupName, roleID string) error

	// Role CRUD
	CreateRole(ctx context.Context, name, description, scopeExpr string, createConstraints models.CreateConstraints, immutableKeys []string, actions []string) (*models.Role, error)
	UpdateRole(ctx context.Context, name string, expectedVersion int, description, scopeExpr string, createConstraints models.CreateConstraints, immutableKeys []string, actions []string) (*models.Role, error)
	DeleteRole(ctx context.Context, name string) error

	// User management
	CreateUser(ctx context.Context, email, username, subject, passwordHash string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserBySubject(ctx context.Context, subject string) (*models.User, error)
	GetUserByID(ctx context.Context, userID string) (*models.User, error)

	// Read-only lookups
	GetRoleByName(ctx context.Context, name string) (*models.Role, error)
	GetRoleByID(ctx context.Context, roleID string) (*models.Role, error)
	ListAllRoles(ctx context.Context) ([]models.Role, error)
	ListGroupRoles(ctx context.Context, groupName *string) ([]models.GroupRole, error)
	GetPrincipalRoles(ctx context.Context, principalID, principalType string) ([]string, error)
	GetRolePermissions(ctx context.Context, roleName string) ([][]string, error)

	// Authorization
	Authorize(ctx context.Context, principal *iam.Principal, obj, act string, labels map[string]interface{}) (bool, error)

	// Cache management
	RefreshGroupRoleCache(ctx context.Context) error
	GetGroupRoleCacheSnapshot() iam.GroupRoleSnapshot
}

// Compile-time assertion: iam.Service must implement iamAdminService.
// This will cause a build failure if iam.Service is missing any required method.
var _ iamAdminService = (iam.Service)(nil)
