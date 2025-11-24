package auth

// Action constants for authorization checks
// These constants define all possible actions in the Grid API for use with Casbin policies
//
// Reference: plan.md §315-331, specs/006-authz-authn-rbac/authorization-design.md (Actions Taxonomy section)

// Control Plane Actions (state management)
const (
	// StateCreate allows creating new states
	StateCreate = "state:create"

	// StateRead allows reading state metadata
	StateRead = "state:read"

	// StateList allows listing states
	StateList = "state:list"

	// StateUpdateLabels allows updating state labels
	StateUpdateLabels = "state:update-labels"

	// StateDelete allows deleting states
	StateDelete = "state:delete"
)

// Data Plane Actions (Terraform HTTP backend)
const (
	// TfstateRead allows reading Terraform state content
	TfstateRead = "tfstate:read"

	// TfstateWrite allows writing Terraform state content
	TfstateWrite = "tfstate:write"

	// TfstateLock allows locking a state
	TfstateLock = "tfstate:lock"

	// TfstateUnlock allows unlocking a state
	TfstateUnlock = "tfstate:unlock"
)

// Dependency Actions
const (
	// DependencyCreate allows creating dependencies
	DependencyCreate = "dependency:create"

	// DependencyRead allows reading dependency metadata
	DependencyRead = "dependency:read"

	// DependencyList allows listing dependencies for a specific state (state-scoped)
	DependencyList = "dependency:list"

	// DependencyListAll allows listing all edges in the system (globally allowed, filtered by handler)
	DependencyListAll = "dependency:list-all"

	// DependencyDelete allows deleting dependencies
	DependencyDelete = "dependency:delete"
)

// State Output Actions
const (
	// StateOutputList allows listing available output keys from a state
	StateOutputList = "state-output:list"

	// StateOutputRead allows reading specific output values from a state
	StateOutputRead = "state-output:read"
)

// Policy Actions (label validation)
const (
	// PolicyRead allows reading label policy
	PolicyRead = "policy:read"

	// PolicyWrite allows updating label policy
	PolicyWrite = "policy:write"
)

// Admin Actions (RBAC management)
const (
	// AdminRoleManage allows creating/updating/deleting roles
	AdminRoleManage = "admin:role-manage"

	// AdminUserAssign allows assigning/removing roles to/from users
	AdminUserAssign = "admin:user-assign"

	// AdminGroupAssign allows assigning/removing roles to/from groups
	AdminGroupAssign = "admin:group-assign"

	// AdminServiceAccountManage allows creating/revoking service accounts
	AdminServiceAccountManage = "admin:service-account-manage"

	// AdminSessionRevoke allows revoking sessions
	AdminSessionRevoke = "admin:session-revoke"

	// AdminCacheRefresh allows manually refreshing the group→role cache
	AdminCacheRefresh = "admin:cache-refresh"
)

// Ownership Actions (self-service access)
const (
	// ReadSelf allows a principal to read their own data
	// This is a special action used in the Casbin model's ownership check
	// When r.act == "read-self" && r.sub == r.obj, access is granted
	ReadSelf = "read-self"
)

// Wildcard Actions (used in policies for broad access)
const (
	// StateWildcard grants all state actions
	StateWildcard = "state:*"

	// TfstateWildcard grants all tfstate actions
	TfstateWildcard = "tfstate:*"

	// DependencyWildcard grants all dependency actions
	DependencyWildcard = "dependency:*"

	// StateOutputWildcard grants all state-output actions
	StateOutputWildcard = "state-output:*"

	// PolicyWildcard grants all policy actions
	PolicyWildcard = "policy:*"

	// AdminWildcard grants all admin actions
	AdminWildcard = "admin:*"

	// AllWildcard grants all actions (platform-engineer)
	AllWildcard = "*"
)

// Object Types for Casbin policies
const (
	// ObjectTypeState represents state resources
	ObjectTypeState = "state"

	// ObjectTypePolicy represents policy resources
	ObjectTypePolicy = "policy"

	// ObjectTypeAdmin represents administrative resources
	ObjectTypeAdmin = "admin"

	// ObjectTypeAll is a wildcard for all object types
	ObjectTypeAll = "*"
)

// ValidateAction checks if an action string is valid
// This prevents typos when creating/updating policies
func ValidateAction(action string) bool {
	validActions := map[string]bool{
		// Control Plane
		StateCreate:       true,
		StateRead:         true,
		StateList:         true,
		StateUpdateLabels: true,
		StateDelete:       true,
		// Data Plane
		TfstateRead:   true,
		TfstateWrite:  true,
		TfstateLock:   true,
		TfstateUnlock: true,
		// Dependencies
		DependencyCreate:  true,
		DependencyRead:    true,
		DependencyList:    true,
		DependencyListAll: true,
		DependencyDelete:  true,
		// State Outputs
		StateOutputList: true,
		StateOutputRead: true,
		// Policy
		PolicyRead:  true,
		PolicyWrite: true,
		// Admin
		AdminRoleManage:           true,
		AdminUserAssign:           true,
		AdminGroupAssign:          true,
		AdminServiceAccountManage: true,
		AdminSessionRevoke:        true,
		AdminCacheRefresh:         true,
		// Ownership
		ReadSelf: true,
		// Wildcards
		StateWildcard:       true,
		TfstateWildcard:     true,
		DependencyWildcard:  true,
		StateOutputWildcard: true,
		PolicyWildcard:      true,
		AdminWildcard:       true,
		AllWildcard:         true,
	}

	return validActions[action]
}

// ExpandWildcard expands wildcard actions to their concrete actions
// Example: "state:*" → ["state:create", "state:read", "state:list", "state:update-labels", "state:delete"]
func ExpandWildcard(action string) []string {
	switch action {
	case StateWildcard:
		return []string{StateCreate, StateRead, StateList, StateUpdateLabels, StateDelete}
	case TfstateWildcard:
		return []string{TfstateRead, TfstateWrite, TfstateLock, TfstateUnlock}
	case DependencyWildcard:
		return []string{DependencyCreate, DependencyRead, DependencyList, DependencyListAll, DependencyDelete}
	case StateOutputWildcard:
		return []string{StateOutputList, StateOutputRead}
	case PolicyWildcard:
		return []string{PolicyRead, PolicyWrite}
	case AdminWildcard:
		return []string{AdminRoleManage, AdminUserAssign, AdminGroupAssign, AdminServiceAccountManage, AdminSessionRevoke, AdminCacheRefresh}
	case AllWildcard:
		// Return all concrete actions
		var all []string
		all = append(all, ExpandWildcard(StateWildcard)...)
		all = append(all, ExpandWildcard(TfstateWildcard)...)
		all = append(all, ExpandWildcard(DependencyWildcard)...)
		all = append(all, ExpandWildcard(StateOutputWildcard)...)
		all = append(all, ExpandWildcard(PolicyWildcard)...)
		all = append(all, ExpandWildcard(AdminWildcard)...)
		return all
	default:
		// Not a wildcard, return as-is
		return []string{action}
	}
}
