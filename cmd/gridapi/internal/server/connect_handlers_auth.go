package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Create service account via IAM service
	// TODO: Extract createdBy from Principal in context
	sa, clientSecret, err := h.iamService.CreateServiceAccount(ctx, req.Msg.Name, "")
	if err != nil {
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// List service accounts via IAM service
	sas, err := h.iamService.ListServiceAccounts(ctx)
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Revoke service account via IAM service
	// This handles:
	// - Disabling the service account
	// - Revoking all active sessions
	// - Removing Casbin role assignments (out-of-band mutation)
	if err := h.iamService.RevokeServiceAccount(ctx, req.Msg.ClientId); err != nil {
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Rotate service account secret via IAM service
	// This handles:
	// - Generating new secret
	// - Hashing with bcrypt
	// - Updating database
	// - Returning unhashed secret and rotation timestamp
	clientSecret, rotatedAt, err := h.iamService.RotateServiceAccountSecret(ctx, req.Msg.ClientId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.RotateServiceAccountResponse{
		ClientId:     req.Msg.ClientId,
		ClientSecret: clientSecret,
		RotatedAt:    timestamppb.New(rotatedAt),
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Get role by name to find its ID
	role, err := h.iamService.GetRoleByName(ctx, req.Msg.RoleName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Determine principal type and get principal ID
	var userID, serviceAccountID string
	switch req.Msg.PrincipalType {
	case "user":
		user, err := h.iamService.GetUserBySubject(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		userID = user.ID
	case "service_account":
		sa, err := h.iamService.GetServiceAccountByClientID(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		serviceAccountID = sa.ID
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid principal type: %s", req.Msg.PrincipalType))
	}

	// Delegate to IAM service (handles DB write, Casbin sync, rollback)
	if err := h.iamService.AssignUserRole(ctx, userID, serviceAccountID, role.ID); err != nil {
		// Map known errors to appropriate gRPC codes
		if strings.Contains(err.Error(), "already assigned") {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("role '%s' is already assigned to principal", req.Msg.RoleName))
		}
		return nil, mapServiceError(err)
	}

	return connect.NewResponse(&statev1.AssignRoleResponse{
		Success:    true,
		AssignedAt: timestamppb.New(time.Now()),
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Get role by name to find its ID
	role, err := h.iamService.GetRoleByName(ctx, req.Msg.RoleName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Determine principal type and validate principal exists
	var userID, serviceAccountID string
	switch req.Msg.PrincipalType {
	case "user":
		// PrincipalId contains internal user ID (UUID)
		user, err := h.iamService.GetUserByID(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		userID = user.ID
	case "service_account":
		// PrincipalId contains internal SA ID (UUID)
		sa, err := h.iamService.GetServiceAccountByID(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		serviceAccountID = sa.ID
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid principal type: %s", req.Msg.PrincipalType))
	}

	// Delegate to IAM service (handles DB delete, Casbin removal)
	if err := h.iamService.RemoveUserRole(ctx, userID, serviceAccountID, role.ID); err != nil {
		return nil, mapServiceError(err)
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Get role by name to find its ID
	role, err := h.iamService.GetRoleByName(ctx, req.Msg.RoleName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Delegate to IAM service (handles DB write, Casbin sync, cache refresh, rollback)
	// Note: IAM service currently doesn't track AssignedBy - enhancement for later
	if err := h.iamService.AssignGroupRole(ctx, req.Msg.GroupName, role.ID); err != nil {
		// Map known errors to appropriate gRPC codes
		if strings.Contains(err.Error(), "already assigned") {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("role '%s' is already assigned to group '%s'", req.Msg.RoleName, req.Msg.GroupName))
		}
		return nil, mapServiceError(err)
	}

	return connect.NewResponse(&statev1.AssignGroupRoleResponse{
		Success:    true,
		AssignedAt: timestamppb.New(time.Now()),
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Get role by name to find its ID
	role, err := h.iamService.GetRoleByName(ctx, req.Msg.RoleName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Delegate to IAM service (handles DB delete, Casbin removal, cache refresh)
	if err := h.iamService.RemoveGroupRole(ctx, req.Msg.GroupName, role.ID); err != nil {
		return nil, mapServiceError(err)
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	groupRoles, err := h.iamService.ListGroupRoles(ctx, req.Msg.GroupName)
	if err != nil {
		return nil, mapServiceError(err)
	}

	assignments := make([]*statev1.GroupRoleAssignmentInfo, 0, len(groupRoles))
	for _, gr := range groupRoles {
		role, err := h.iamService.GetRoleByID(ctx, gr.RoleID)
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	var principalID string

	switch req.Msg.PrincipalType {
	case "user":
		user, err := h.iamService.GetUserBySubject(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		principalID = user.ID
	case "service_account":
		sa, err := h.iamService.GetServiceAccountByClientID(ctx, req.Msg.PrincipalId)
		if err != nil {
			return nil, mapServiceError(err)
		}
		principalID = sa.ID
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid principal type: %s", req.Msg.PrincipalType))
	}

	casbinRoles, err := h.iamService.GetPrincipalRoles(ctx, principalID, req.Msg.PrincipalType)
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

		role, err := h.iamService.GetRoleByName(ctx, roleName)
		if err != nil {
			continue // Role might have been deleted
		}

		if role.ScopeExpr != "" {
			labelScopeExprs[role.ScopeExpr] = struct{}{}
		}

		for _, key := range role.ImmutableKeys {
			immutableKeys[key] = struct{}{}
		}

		// Get permissions (actions) for the role from IAM service
		permissions, err := h.iamService.GetRolePermissions(ctx, roleName)
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

	// Map create_constraints from proto to models
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Delegate to IAM service (handles validation, DB create, Casbin sync, rollback)
	role, err := h.iamService.CreateRole(
		ctx,
		req.Msg.Name,
		req.Msg.GetDescription(),
		req.Msg.GetLabelScopeExpr(),
		constraintsMap,
		req.Msg.ImmutableKeys,
		req.Msg.Actions,
	)
	if err != nil {
		// Map known errors to appropriate gRPC codes
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "already exists") {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		if strings.Contains(err.Error(), "invalid label_scope_expr") {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, mapServiceError(err)
	}

	// Convert role to proto
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	roles, err := h.iamService.ListAllRoles(ctx)
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

	// Map create_constraints from proto to models
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Delegate to IAM service (handles validation, optimistic locking, DB update, Casbin sync)
	updatedRole, err := h.iamService.UpdateRole(
		ctx,
		req.Msg.Name,
		int(req.Msg.ExpectedVersion),
		req.Msg.GetDescription(),
		req.Msg.GetLabelScopeExpr(),
		constraintsMap,
		req.Msg.ImmutableKeys,
		req.Msg.Actions,
	)
	if err != nil {
		// Map known errors to appropriate gRPC codes
		if strings.Contains(err.Error(), "version mismatch") || strings.Contains(err.Error(), "modified by another") {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if strings.Contains(err.Error(), "invalid label_scope_expr") {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, mapServiceError(err)
	}

	// Convert role to proto
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	// Delegate to IAM service (handles safety check, DB delete, Casbin cleanup)
	if err := h.iamService.DeleteRole(ctx, req.Msg.Name); err != nil {
		// Map known errors to appropriate gRPC codes
		if strings.Contains(err.Error(), "still assigned") {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, mapServiceError(err)
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	sessions, err := h.iamService.ListUserSessions(ctx, req.Msg.UserId)
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

	if h.iamService == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not available"))
	}

	if err := h.iamService.RevokeSession(ctx, req.Msg.SessionId); err != nil {
		return nil, mapServiceError(err)
	}

	return connect.NewResponse(&statev1.RevokeSessionResponse{Success: true}), nil
}

// roleToProto is a helper to convert a database role model to a protobuf message.
func (h *StateServiceHandler) roleToProto(ctx context.Context, role *models.Role) (*statev1.RoleInfo, error) {
	if h.iamService == nil {
		return nil, fmt.Errorf("IAM service not available")
	}

	permissions, err := h.iamService.GetRolePermissions(ctx, role.Name)
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
