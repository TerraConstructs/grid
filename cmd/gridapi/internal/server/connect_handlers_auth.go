package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"

	"github.com/hashicorp/go-bexpr"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateServiceAccount creates a new service account.
func (h *StateServiceHandler) CreateServiceAccount(
	ctx context.Context,
	req *connect.Request[statev1.CreateServiceAccountRequest],
) (*connect.Response[statev1.CreateServiceAccountResponse], error) {
	if !h.cfg.OIDC.IsInternalIdPMode() {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("service accounts can only be managed when running in internal IdP mode"))
	}

	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go

	// Generate client_id and client_secret
	clientID := uuid.Must(uuid.NewV7()).String()
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate secret: %w", err))
	}
	clientSecret := hex.EncodeToString(secretBytes)

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash secret: %w", err))
	}

	sa := &models.ServiceAccount{
		Name:             req.Msg.Name,
		Description:      req.Msg.GetDescription(),
		ClientID:         clientID,
		ClientSecretHash: string(hashedSecret),
	}

	if err := h.authnDeps.ServiceAccounts.Create(ctx, sa); err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.CreateServiceAccountResponse{
		Id:           sa.ID,
		ClientId:     sa.ClientID,
		ClientSecret: clientSecret,
		Name:         sa.Name,
		CreatedAt:    timestamppb.New(sa.CreatedAt),
	}

	return connect.NewResponse(resp), nil
}

// ListServiceAccounts lists all service accounts.
func (h *StateServiceHandler) ListServiceAccounts(
	ctx context.Context,
	req *connect.Request[statev1.ListServiceAccountsRequest],
) (*connect.Response[statev1.ListServiceAccountsResponse], error) {
	if !h.cfg.OIDC.IsInternalIdPMode() {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("service accounts can only be managed when running in internal IdP mode"))
	}

	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go

	sas, err := h.authnDeps.ServiceAccounts.List(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.ListServiceAccountsResponse{
		ServiceAccounts: make([]*statev1.ServiceAccountInfo, len(sas)),
	}

	for i, sa := range sas {
		resp.ServiceAccounts[i] = &statev1.ServiceAccountInfo{
			Id:          sa.ID,
			ClientId:    sa.ClientID,
			Name:        sa.Name,
			Description: &sa.Description,
			CreatedAt:   timestamppb.New(sa.CreatedAt),
			LastUsedAt:  timestamppb.New(sa.LastUsedAt),
			Disabled:    sa.Disabled,
		}
	}

	return connect.NewResponse(resp), nil
}

// RevokeServiceAccount revokes a service account.
func (h *StateServiceHandler) RevokeServiceAccount(
	ctx context.Context,
	req *connect.Request[statev1.RevokeServiceAccountRequest],
) (*connect.Response[statev1.RevokeServiceAccountResponse], error) {
	if !h.cfg.OIDC.IsInternalIdPMode() {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("service accounts can only be managed when running in internal IdP mode"))
	}

	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go

	sa, err := h.authnDeps.ServiceAccounts.GetByClientID(ctx, req.Msg.ClientId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	if err := h.authnDeps.ServiceAccounts.SetDisabled(ctx, sa.ID, true); err != nil {
		return nil, mapServiceError(err)
	}

	// Remove Casbin role assignments for the service account
	casbinID := auth.ServiceAccountID(sa.ID)
	if _, err := h.authnDeps.Enforcer.DeleteRolesForUser(casbinID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete casbin roles: %w", err))
	}

	if err := h.authnDeps.Sessions.RevokeByServiceAccountID(ctx, sa.ID); err != nil {
		return nil, mapServiceError(err)
	}

	return connect.NewResponse(&statev1.RevokeServiceAccountResponse{Success: true}), nil
}

// RotateServiceAccount rotates a service account's secret.
func (h *StateServiceHandler) RotateServiceAccount(
	ctx context.Context,
	req *connect.Request[statev1.RotateServiceAccountRequest],
) (*connect.Response[statev1.RotateServiceAccountResponse], error) {
	if !h.cfg.OIDC.IsInternalIdPMode() {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("service accounts can only be managed when running in internal IdP mode"))
	}

	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:service-account-manage)

	sa, err := h.authnDeps.ServiceAccounts.GetByClientID(ctx, req.Msg.ClientId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate secret: %w", err))
	}
	clientSecret := hex.EncodeToString(secretBytes)

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash secret: %w", err))
	}

	if err := h.authnDeps.ServiceAccounts.UpdateSecretHash(ctx, sa.ID, string(hashedSecret)); err != nil {
		return nil, mapServiceError(err)
	}

	updatedSa, err := h.authnDeps.ServiceAccounts.GetByID(ctx, sa.ID)
	if err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.RotateServiceAccountResponse{
		ClientId:     updatedSa.ClientID,
		ClientSecret: clientSecret,
		RotatedAt:    timestamppb.New(updatedSa.SecretRotatedAt),
	}

	return connect.NewResponse(resp), nil
}

// AssignRole assigns a role to a user or service account.
func (h *StateServiceHandler) AssignRole(
	ctx context.Context,
	req *connect.Request[statev1.AssignRoleRequest],
) (*connect.Response[statev1.AssignRoleResponse], error) {

	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:user-assign)

	// Get role by name to find its ID
	role, err := h.authnDeps.Roles.GetByName(ctx, req.Msg.RoleName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	var casbinPrincipalID string
	userRole := &models.UserRole{
		RoleID: role.ID,
	}

	switch req.Msg.PrincipalType {
	case "user":
		user, err := h.authnDeps.Users.GetBySubject(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		userRole.UserID = &user.ID
		subjectID := user.PrincipalSubject()
		casbinPrincipalID = auth.UserID(subjectID)
	case "service_account":
		sa, err := h.authnDeps.ServiceAccounts.GetByClientID(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		userRole.ServiceAccountID = &sa.ID
		casbinPrincipalID = auth.ServiceAccountID(sa.ClientID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid principal type: %s", req.Msg.PrincipalType))
	}

	// Persist the assignment for audit/query purposes
	if err := h.authnDeps.UserRoles.Create(ctx, userRole); err != nil {
		// Handle potential unique constraint violation gracefully
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("role '%s' is already assigned to principal", req.Msg.RoleName))
		}
		return nil, mapServiceError(err)
	}

	// Add the grouping to Casbin for enforcement
	casbinRoleID := auth.RoleID(role.Name)
	if _, err := h.authnDeps.Enforcer.AddRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
		// Attempt to roll back the database change if Casbin fails
		_ = h.authnDeps.UserRoles.Delete(ctx, userRole.ID)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add casbin role assignment: %w", err))
	}

	return connect.NewResponse(&statev1.AssignRoleResponse{
		Success:    true,
		AssignedAt: timestamppb.New(userRole.AssignedAt),
	}), nil
}

// RemoveRole removes a role from a user or service account.
func (h *StateServiceHandler) RemoveRole(
	ctx context.Context,
	req *connect.Request[statev1.RemoveRoleRequest],
) (*connect.Response[statev1.RemoveRoleResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:user-assign)

	// Get role by name to find its ID
	role, err := h.authnDeps.Roles.GetByName(ctx, req.Msg.RoleName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	var casbinPrincipalID string

	switch req.Msg.PrincipalType {
	case "user":
		user, err := h.authnDeps.Users.GetByID(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		if err := h.authnDeps.UserRoles.DeleteByUserAndRole(ctx, user.ID, role.ID); err != nil {
			return nil, mapServiceError(err)
		}
		subjectID := user.PrincipalSubject()
		casbinPrincipalID = auth.UserID(subjectID)
	case "service_account":
		sa, err := h.authnDeps.ServiceAccounts.GetByID(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		if err := h.authnDeps.UserRoles.DeleteByServiceAccountAndRole(ctx, sa.ID, role.ID); err != nil {
			return nil, mapServiceError(err)
		}
		casbinPrincipalID = auth.ServiceAccountID(sa.ClientID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid principal type: %s", req.Msg.PrincipalType))
	}

	// Remove the grouping from Casbin
	casbinRoleID := auth.RoleID(role.Name)
	if _, err := h.authnDeps.Enforcer.DeleteRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove casbin role assignment: %w", err))
	}

	return connect.NewResponse(&statev1.RemoveRoleResponse{Success: true}), nil
}

// AssignGroupRole assigns a role to a group.
func (h *StateServiceHandler) AssignGroupRole(
	ctx context.Context,
	req *connect.Request[statev1.AssignGroupRoleRequest],
) (*connect.Response[statev1.AssignGroupRoleResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:group-assign)

	// Get authenticated principal for audit trail
	principal, ok := auth.GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("no authenticated principal"))
	}

	// Get role by name to find its ID
	role, err := h.authnDeps.Roles.GetByName(ctx, req.Msg.RoleName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	groupRole := &models.GroupRole{
		GroupName:  req.Msg.GroupName,
		RoleID:     role.ID,
		AssignedBy: principal.InternalID,
	}

	// Persist the assignment for audit/query purposes
	if err := h.authnDeps.GroupRoles.Create(ctx, groupRole); err != nil {
		// Handle potential unique constraint violation gracefully
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("role '%s' is already assigned to group '%s'", req.Msg.RoleName, req.Msg.GroupName))
		}
		return nil, mapServiceError(err)
	}

	// Add the grouping to Casbin for enforcement
	casbinPrincipalID := auth.GroupID(req.Msg.GroupName)
	casbinRoleID := auth.RoleID(role.Name)
	if _, err := h.authnDeps.Enforcer.AddRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
		// Attempt to roll back the database change if Casbin fails
		_ = h.authnDeps.GroupRoles.Delete(ctx, groupRole.ID)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add casbin group-role assignment: %w", err))
	}

	return connect.NewResponse(&statev1.AssignGroupRoleResponse{
		Success:    true,
		AssignedAt: timestamppb.New(groupRole.AssignedAt),
	}), nil
}

// RemoveGroupRole removes a role from a group.
func (h *StateServiceHandler) RemoveGroupRole(
	ctx context.Context,
	req *connect.Request[statev1.RemoveGroupRoleRequest],
) (*connect.Response[statev1.RemoveGroupRoleResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:group-assign)

	// Get role by name to find its ID
	role, err := h.authnDeps.Roles.GetByName(ctx, req.Msg.RoleName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Delete from the database
	if err := h.authnDeps.GroupRoles.DeleteByGroupAndRole(ctx, req.Msg.GroupName, role.ID); err != nil {
		return nil, mapServiceError(err)
	}

	// Remove the grouping from Casbin
	casbinPrincipalID := auth.GroupID(req.Msg.GroupName)
	casbinRoleID := auth.RoleID(role.Name)
	if _, err := h.authnDeps.Enforcer.DeleteRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove casbin group-role assignment: %w", err))
	}

	return connect.NewResponse(&statev1.RemoveGroupRoleResponse{Success: true}), nil
}

// ListGroupRoles lists roles assigned to groups.
func (h *StateServiceHandler) ListGroupRoles(
	ctx context.Context,
	req *connect.Request[statev1.ListGroupRolesRequest],
) (*connect.Response[statev1.ListGroupRolesResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:group-assign)

	var groupRoles []models.GroupRole
	var err error

	if req.Msg.GroupName != nil {
		groupRoles, err = h.authnDeps.GroupRoles.GetByGroupName(ctx, *req.Msg.GroupName)
	} else {
		groupRoles, err = h.authnDeps.GroupRoles.List(ctx)
	}

	if err != nil {
		return nil, mapServiceError(err)
	}

	assignments := make([]*statev1.GroupRoleAssignmentInfo, 0, len(groupRoles))
	for _, gr := range groupRoles {
		role, err := h.authnDeps.Roles.GetByID(ctx, gr.RoleID)
		if err != nil {
			// Inconsistent data, log it but continue if possible
			continue
		}

		// The AssignedBy field in the DB stores a user ID. We need to fetch the user to get a displayable name/email.
		// For simplicity in this example, we'll return the ID directly.
		// In a real app, you might want to fetch the user details.
		assignments = append(assignments, &statev1.GroupRoleAssignmentInfo{
			GroupName:        gr.GroupName,
			RoleName:         role.Name,
			AssignedAt:       timestamppb.New(gr.AssignedAt),
			AssignedByUserId: gr.AssignedBy, // Returning ID directly
		})
	}

	return connect.NewResponse(&statev1.ListGroupRolesResponse{Assignments: assignments}), nil
}

// GetEffectivePermissions returns the aggregated permissions for a principal.
func (h *StateServiceHandler) GetEffectivePermissions(
	ctx context.Context,
	req *connect.Request[statev1.GetEffectivePermissionsRequest],
) (*connect.Response[statev1.GetEffectivePermissionsResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (current user or admin)

	var casbinPrincipalID string

	switch req.Msg.PrincipalType {
	case "user":
		user, err := h.authnDeps.Users.GetBySubject(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		subjectID := user.PrincipalSubject()
		casbinPrincipalID = auth.UserID(subjectID)
	case "service_account":
		sa, err := h.authnDeps.ServiceAccounts.GetByClientID(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		casbinPrincipalID = auth.ServiceAccountID(sa.ClientID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid principal type: %s", req.Msg.PrincipalType))
	}

	if h.authnDeps.Enforcer == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("casbin enforcer not initialized"))
	}
	casbinRoles, err := h.authnDeps.Enforcer.GetRolesForUser(casbinPrincipalID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get roles for user: %w", err))
	}

	// Use maps to collect unique values
	actions := make(map[string]struct{})
	labelScopeExprs := make(map[string]struct{})
	immutableKeys := make(map[string]struct{})

	for _, casbinRole := range casbinRoles {
		roleName, err := auth.ExtractRoleID(casbinRole)
		if err != nil {
			continue // Should not happen if prefixes are consistent
		}

		role, err := h.authnDeps.Roles.GetByName(ctx, roleName)
		if err != nil {
			continue // Role might have been deleted
		}

		if role.ScopeExpr != "" {
			labelScopeExprs[role.ScopeExpr] = struct{}{}
		}

		for _, key := range role.ImmutableKeys {
			immutableKeys[key] = struct{}{}
		}

		// Get permissions (actions) for the role from Casbin
		permissions, err := h.authnDeps.Enforcer.GetPermissionsForUser(casbinRole)
		if err != nil {
			continue // TODO: Don't ignore error (use multierr?)
		}
		for _, p := range permissions {
			if len(p) > 2 { // p = [role, objType, action, ...]
				actions[p[2]] = struct{}{}
			}
		}
	}

	// Convert maps to slices
	finalActions := make([]string, 0, len(actions))
	for action := range actions {
		finalActions = append(finalActions, action)
	}
	sort.Strings(finalActions)

	finalScopes := make([]string, 0, len(labelScopeExprs))
	for scope := range labelScopeExprs {
		finalScopes = append(finalScopes, scope)
	}
	sort.Strings(finalScopes)

	finalImmutableKeys := make([]string, 0, len(immutableKeys))
	for key := range immutableKeys {
		finalImmutableKeys = append(finalImmutableKeys, key)
	}
	sort.Strings(finalImmutableKeys)

	resp := &statev1.GetEffectivePermissionsResponse{
		Permissions: &statev1.EffectivePermissions{
			Roles:                  casbinRoles,
			Actions:                finalActions,
			LabelScopeExprs:        finalScopes,
			EffectiveImmutableKeys: finalImmutableKeys,
			// TODO: Aggregate CreateConstraints
		},
	}

	return connect.NewResponse(resp), nil
}

// CreateRole creates a new role.
func (h *StateServiceHandler) CreateRole(
	ctx context.Context,
	req *connect.Request[statev1.CreateRoleRequest],
) (*connect.Response[statev1.CreateRoleResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:role-manage)

	// Validate label_scope_expr as valid go-bexpr syntax
	if expr := req.Msg.GetLabelScopeExpr(); expr != "" {
		if _, err := bexpr.CreateEvaluator(expr); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid label_scope_expr: %w", err))
		}
	}

	// Map create_constraints
	var constraintsMap models.CreateConstraints
	if req.Msg.GetCreateConstraints() != nil && len(req.Msg.GetCreateConstraints().GetConstraints()) > 0 {
		protoConstraints := req.Msg.GetCreateConstraints().GetConstraints()
		constraintsMap = make(models.CreateConstraints, len(protoConstraints))
		for k, v := range protoConstraints {
			constraintsMap[k] = models.CreateConstraint{
				AllowedValues: v.AllowedValues,
				Required:      v.Required,
			}
		}
	}

	role := &models.Role{
		Name:              req.Msg.Name,
		Description:       req.Msg.GetDescription(),
		ScopeExpr:         req.Msg.GetLabelScopeExpr(),
		CreateConstraints: constraintsMap,
		ImmutableKeys:     req.Msg.ImmutableKeys,
	}

	if err := h.authnDeps.Roles.Create(ctx, role); err != nil {
		return nil, mapServiceError(err)
	}

	casbinRoleID := auth.RoleID(role.Name)
	for _, action := range req.Msg.Actions {
		parts := strings.Split(action, ":")
		if len(parts) != 2 {
			// For simplicity, we only support obj:act format. Wildcards can be handled if needed.
			continue
		}
		objType, act := parts[0], parts[1]
		// The policy format is [role, objType, action, scopeExpr, effect]
		// We store the scope expression directly in the policy to use bexprMatch in the model.
		policy := []string{casbinRoleID, objType, act, role.ScopeExpr, "allow"}
		if _, err := h.authnDeps.Enforcer.AddPolicy(policy); err != nil {
			// Attempt to roll back the database change if Casbin fails
			_ = h.authnDeps.Roles.Delete(ctx, role.ID)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add casbin policy: %w", err))
		}
	}

	roleInfo, err := h.roleToProto(ctx, role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert role to proto: %w", err))
	}

	return connect.NewResponse(&statev1.CreateRoleResponse{Role: roleInfo}), nil
}

// ListRoles lists all available roles.
func (h *StateServiceHandler) ListRoles(
	ctx context.Context,
	req *connect.Request[statev1.ListRolesRequest],
) (*connect.Response[statev1.ListRolesResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:role-manage)

	roles, err := h.authnDeps.Roles.List(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}

	roleInfos := make([]*statev1.RoleInfo, 0, len(roles))
	for _, role := range roles {
		roleInfo, err := h.roleToProto(ctx, &role)
		if err != nil {
			// Log the error but continue, so one bad role doesn't break the whole list
			// logger.Error("Failed to convert role to proto", "role", role.Name, "error", err)
			continue
		}
		roleInfos = append(roleInfos, roleInfo)
	}

	return connect.NewResponse(&statev1.ListRolesResponse{Roles: roleInfos}), nil
}

// UpdateRole updates an existing role.
func (h *StateServiceHandler) UpdateRole(
	ctx context.Context,
	req *connect.Request[statev1.UpdateRoleRequest],
) (*connect.Response[statev1.UpdateRoleResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:role-manage)

	// Validate label_scope_expr
	if expr := req.Msg.GetLabelScopeExpr(); expr != "" {
		if _, err := bexpr.CreateEvaluator(expr); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid label_scope_expr: %w", err))
		}
	}

	// Get the existing role
	role, err := h.authnDeps.Roles.GetByName(ctx, req.Msg.Name)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Optimistic locking
	if role.Version != int(req.Msg.ExpectedVersion) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("role has been modified by another user, please refresh and try again"))
	}

	// Map create_constraints
	var constraintsMap models.CreateConstraints
	if req.Msg.GetCreateConstraints() != nil && len(req.Msg.GetCreateConstraints().GetConstraints()) > 0 {
		protoConstraints := req.Msg.GetCreateConstraints().GetConstraints()
		constraintsMap = make(models.CreateConstraints, len(protoConstraints))
		for k, v := range protoConstraints {
			constraintsMap[k] = models.CreateConstraint{
				AllowedValues: v.AllowedValues,
				Required:      v.Required,
			}
		}
	}

	// Update role fields
	role.Description = req.Msg.GetDescription()
	role.ScopeExpr = req.Msg.GetLabelScopeExpr()
	role.CreateConstraints = constraintsMap
	role.ImmutableKeys = req.Msg.ImmutableKeys

	if err := h.authnDeps.Roles.Update(ctx, role); err != nil {
		return nil, mapServiceError(err)
	}

	// Sync Casbin policies
	casbinRoleID := auth.RoleID(role.Name)
	if _, err := h.authnDeps.Enforcer.RemoveFilteredPolicy(0, casbinRoleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove old casbin policies: %w", err))
	}

	for _, action := range req.Msg.Actions {
		parts := strings.Split(action, ":")
		if len(parts) != 2 {
			continue
		}
		objType, act := parts[0], parts[1]
		policy := []string{casbinRoleID, objType, act, role.ScopeExpr, "allow"}
		if _, err := h.authnDeps.Enforcer.AddPolicy(policy); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add new casbin policy: %w", err))
		}
	}

	updatedRole, err := h.authnDeps.Roles.GetByID(ctx, role.ID)
	if err != nil {
		return nil, mapServiceError(err)
	}

	roleInfo, err := h.roleToProto(ctx, updatedRole)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert role to proto: %w", err))
	}

	return connect.NewResponse(&statev1.UpdateRoleResponse{Role: roleInfo}), nil
}

// DeleteRole deletes a role.
func (h *StateServiceHandler) DeleteRole(
	ctx context.Context,
	req *connect.Request[statev1.DeleteRoleRequest],
) (*connect.Response[statev1.DeleteRoleResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (admin:role-manage)

	role, err := h.authnDeps.Roles.GetByName(ctx, req.Msg.Name)
	if err != nil {
		return nil, mapServiceError(err)
	}

	casbinRoleID := auth.RoleID(role.Name)
	users, err := h.authnDeps.Enforcer.GetUsersForRole(casbinRoleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check role assignments: %w", err))
	}
	if len(users) > 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot delete role: still assigned to %d principals", len(users)))
	}

	// Delete from DB
	if err := h.authnDeps.Roles.Delete(ctx, role.ID); err != nil {
		return nil, mapServiceError(err)
	}

	// Remove policies from Casbin
	if _, err := h.authnDeps.Enforcer.RemoveFilteredPolicy(0, casbinRoleID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove casbin policies: %w", err))
	}

	return connect.NewResponse(&statev1.DeleteRoleResponse{Success: true}), nil
}

// ListSessions lists active sessions for a user.
func (h *StateServiceHandler) ListSessions(
	ctx context.Context,
	req *connect.Request[statev1.ListSessionsRequest],
) (*connect.Response[statev1.ListSessionsResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (user can list their own, admin can list any)

	sessions, err := h.authnDeps.Sessions.GetByUserID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	sessionInfos := make([]*statev1.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		info := &statev1.SessionInfo{
			Id:         s.ID,
			CreatedAt:  timestamppb.New(s.CreatedAt),
			ExpiresAt:  timestamppb.New(s.ExpiresAt),
			LastUsedAt: timestamppb.New(s.LastUsedAt),
			UserAgent:  s.UserAgent,
			IpAddress:  s.IPAddress,
		}
		sessionInfos = append(sessionInfos, info)
	}

	return connect.NewResponse(&statev1.ListSessionsResponse{Sessions: sessionInfos}), nil
}

// RevokeSession revokes a specific session.
func (h *StateServiceHandler) RevokeSession(
	ctx context.Context,
	req *connect.Request[statev1.RevokeSessionRequest],
) (*connect.Response[statev1.RevokeSessionResponse], error) {
	// NOTE: Authz is handled by interceptors middleware
	// cmd/gridapi/internal/middleware/authz_interceptor.go
	// authorization check (user can revoke their own, admin can revoke any)

	if err := h.authnDeps.Sessions.Revoke(ctx, req.Msg.SessionId); err != nil {
		return nil, mapServiceError(err)
	}

	return connect.NewResponse(&statev1.RevokeSessionResponse{Success: true}), nil
}

// roleToProto is a helper to convert a database role model to a protobuf message.
func (h *StateServiceHandler) roleToProto(ctx context.Context, role *models.Role) (*statev1.RoleInfo, error) {
	casbinRoleID := auth.RoleID(role.Name)
	permissions, err := h.authnDeps.Enforcer.GetPermissionsForUser(casbinRoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions for role: %w", err)
	}

	// Extract actions from permissions
	// permissions are in the form [role, objType, action, ...]
	// We want to return them as "objType:action"
	actions := make([]string, 0, len(permissions))
	for _, p := range permissions {
		if len(p) >= 3 { // p = [role, objType, action, ...]
			actions = append(actions, fmt.Sprintf("%s:%s", p[1], p[2]))
		}
	}
	sort.Strings(actions)

	// Convert create_constraints
	var protoConstraints *statev1.CreateConstraints
	if len(role.CreateConstraints) > 0 {
		newMap := make(map[string]*statev1.CreateConstraint, len(role.CreateConstraints))
		for k, v := range role.CreateConstraints {
			newMap[k] = &statev1.CreateConstraint{
				AllowedValues: v.AllowedValues,
				Required:      v.Required,
			}
		}
		protoConstraints = &statev1.CreateConstraints{
			Constraints: newMap,
		}
	}

	return &statev1.RoleInfo{
		Id:                role.ID,
		Name:              role.Name,
		Description:       &role.Description,
		Actions:           actions,
		LabelScopeExpr:    &role.ScopeExpr,
		CreateConstraints: protoConstraints,
		ImmutableKeys:     role.ImmutableKeys,
		CreatedAt:         timestamppb.New(role.CreatedAt),
		UpdatedAt:         timestamppb.New(role.UpdatedAt),
		Version:           int32(role.Version),
	}, nil
}
