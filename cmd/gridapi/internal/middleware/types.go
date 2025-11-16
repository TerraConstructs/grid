package middleware

import (
	"github.com/casbin/casbin/v2"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// AuthnDependencies provides repository access for authentication operations.
//
// Phase 6 Note: This struct is deprecated and will be removed in Phase 6.
// It's currently still used by auth handlers (HandleInternalLogin, HandleSSOCallback,
// HandleWhoAmI, HandleLogout) which will be refactored to use the IAM service instead.
//
// New code should use the IAM service (services/iam) instead of this struct.
type AuthnDependencies struct {
	Sessions        repository.SessionRepository
	Users           repository.UserRepository
	UserRoles       repository.UserRoleRepository
	ServiceAccounts repository.ServiceAccountRepository
	RevokedJTIs     repository.RevokedJTIRepository
	GroupRoles      repository.GroupRoleRepository
	Roles           repository.RoleRepository
	Enforcer        casbin.IEnforcer
}
