package server

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	gridmiddleware "github.com/terraconstructs/grid/cmd/gridapi/internal/middleware"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/dependency"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/services/state"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StateServiceHandler wires the internal state service to Connect RPC contracts.
type StateServiceHandler struct {
	statev1connect.UnimplementedStateServiceHandler
	service       *statepkg.Service
	depService    *dependency.Service
	policyService *statepkg.PolicyService
	iamService    iamAdminService // Compile-time verified IAM service contract
	authnDeps     *gridmiddleware.AuthnDependencies
	cfg           *config.Config
}

// NewStateServiceHandler constructs a handler backed by the provided service.
func NewStateServiceHandler(service *statepkg.Service, authnDeps *gridmiddleware.AuthnDependencies, cfg *config.Config) *StateServiceHandler {
	return &StateServiceHandler{service: service, authnDeps: authnDeps, cfg: cfg}
}

// WithPolicyService adds the policy service to the handler (optional dependency).
func (h *StateServiceHandler) WithPolicyService(policyService *statepkg.PolicyService) *StateServiceHandler {
	h.policyService = policyService
	return h
}

// WithIAMService adds the IAM service to the handler (optional dependency).
// Used to refresh the group→role cache after admin operations.
func (h *StateServiceHandler) WithIAMService(iamService iamAdminService) *StateServiceHandler {
	h.iamService = iamService
	return h
}

// CreateState creates a new state with client-generated GUID and logic_id.
func (h *StateServiceHandler) CreateState(
	ctx context.Context,
	req *connect.Request[statev1.CreateStateRequest],
) (*connect.Response[statev1.CreateStateResponse], error) {
	// Convert proto labels to models.LabelMap
	labels := make(models.LabelMap)
	if req.Msg.Labels != nil {
		for k, v := range req.Msg.Labels {
			labels[k] = v
		}
	}

	summary, config, err := h.service.CreateState(ctx, req.Msg.Guid, req.Msg.LogicId, labels)
	if err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.CreateStateResponse{
		Guid:    summary.GUID,
		LogicId: summary.LogicID,
		BackendConfig: &statev1.BackendConfig{
			Address:       config.Address,
			LockAddress:   config.LockAddress,
			UnlockAddress: config.UnlockAddress,
		},
	}

	return connect.NewResponse(resp), nil
}

// ListStates returns all states with summary information.
// T038: Updated to support filter parameter and include_labels toggle (FR-020a).
// States are filtered based on the user's role label scopes to ensure users only see states they have access to.
func (h *StateServiceHandler) ListStates(
	ctx context.Context,
	req *connect.Request[statev1.ListStatesRequest],
) (*connect.Response[statev1.ListStatesResponse], error) {
	// Get filter from request (optional)
	filter := ""
	if req.Msg.Filter != nil {
		filter = *req.Msg.Filter
	}

	// Determine if labels should be included (default: true per FR-020a)
	includeLabels := true
	if req.Msg.IncludeLabels != nil {
		includeLabels = *req.Msg.IncludeLabels
	}

	// TODO: After regenerating proto with buf generate, uncomment this
	// Determine if status should be computed (default: false to avoid N+1 queries)
	includeStatus := false
	// if req.Msg.IncludeStatus != nil {
	// 	includeStatus = *req.Msg.IncludeStatus
	// }

	// Get states - use ListWithFilter if filter provided
	var summaries []statepkg.StateSummary
	var err error

	if filter != "" {
		summaries, err = h.service.ListStatesWithFilter(ctx, filter, 1000, 0)
	} else {
		summaries, err = h.service.ListStates(ctx)
	}
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Filter states based on user's role label scopes
	// This ensures users only see states they have access to
	filteredSummaries, err := h.filterStatesByRoleScopes(ctx, summaries)
	if err != nil {
		return nil, mapServiceError(err)
	}

	infos := make([]*statev1.StateInfo, 0, len(filteredSummaries))
	for _, summary := range filteredSummaries {
		info := summaryToProto(summary)

		// Add labels if requested
		if includeLabels {
			info.Labels = make(map[string]*statev1.LabelValue, len(summary.Labels))
			for k, v := range summary.Labels {
				info.Labels[k] = goValueToProtoLabel(v)
			}
		}

		// Populate computed_status and dependency_logic_ids ONLY if explicitly requested
		// (avoids N+1 queries for status computation)
		if includeStatus && h.depService != nil {
			status, err := h.depService.GetStateStatus(ctx, summary.LogicID, "")
			if err == nil && status != nil {
				info.ComputedStatus = &status.Status
			}

			edges, err := h.depService.ListDependencies(ctx, summary.LogicID, "")
			if err == nil {
				logicIDSet := make(map[string]struct{})
				for _, edge := range edges {
					fromState, err := h.service.GetStateByGUID(ctx, edge.FromState)
					if err != nil || fromState == nil {
						continue
					}
					logicIDSet[fromState.LogicID] = struct{}{}
				}

				if len(logicIDSet) > 0 {
					logicIDs := make([]string, 0, len(logicIDSet))
					for logicID := range logicIDSet {
						logicIDs = append(logicIDs, logicID)
					}
					sort.Strings(logicIDs)
					info.DependencyLogicIds = logicIDs
				}
			}
		}

		infos = append(infos, info)
	}

	resp := &statev1.ListStatesResponse{States: infos}
	return connect.NewResponse(resp), nil
}

// GetStateConfig retrieves backend configuration for an existing state by logic_id.
func (h *StateServiceHandler) GetStateConfig(
	ctx context.Context,
	req *connect.Request[statev1.GetStateConfigRequest],
) (*connect.Response[statev1.GetStateConfigResponse], error) {
	guid, config, err := h.service.GetStateConfig(ctx, req.Msg.LogicId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.GetStateConfigResponse{
		Guid: guid,
		BackendConfig: &statev1.BackendConfig{
			Address:       config.Address,
			LockAddress:   config.LockAddress,
			UnlockAddress: config.UnlockAddress,
		},
	}

	return connect.NewResponse(resp), nil
}

// GetStateLock inspects the current lock metadata for a state by GUID.
func (h *StateServiceHandler) GetStateLock(
	ctx context.Context,
	req *connect.Request[statev1.GetStateLockRequest],
) (*connect.Response[statev1.GetStateLockResponse], error) {
	lockInfo, err := h.service.GetStateLock(ctx, req.Msg.Guid)
	if err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.GetStateLockResponse{Lock: lockInfoToProto(lockInfo)}
	return connect.NewResponse(resp), nil
}

// UnlockState releases a lock using the provided lock ID.
func (h *StateServiceHandler) UnlockState(
	ctx context.Context,
	req *connect.Request[statev1.UnlockStateRequest],
) (*connect.Response[statev1.UnlockStateResponse], error) {
	if err := h.service.UnlockState(ctx, req.Msg.Guid, req.Msg.LockId); err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.UnlockStateResponse{Lock: &statev1.StateLock{Locked: false}}
	return connect.NewResponse(resp), nil
}

func summaryToProto(summary statepkg.StateSummary) *statev1.StateInfo {
	info := &statev1.StateInfo{
		Guid:      summary.GUID,
		LogicId:   summary.LogicID,
		Locked:    summary.Locked,
		SizeBytes: summary.SizeBytes,
	}
	if !summary.CreatedAt.IsZero() {
		info.CreatedAt = timestamppb.New(summary.CreatedAt)
	}
	if !summary.UpdatedAt.IsZero() {
		info.UpdatedAt = timestamppb.New(summary.UpdatedAt)
	}
	return info
}

func lockInfoToProto(lockInfo *models.LockInfo) *statev1.StateLock {
	if lockInfo == nil {
		return &statev1.StateLock{Locked: false}
	}

	protoLock := &statev1.StateLock{Locked: true, Info: &statev1.LockInfo{
		Id:        lockInfo.ID,
		Operation: lockInfo.Operation,
		Info:      lockInfo.Info,
		Who:       lockInfo.Who,
		Version:   lockInfo.Version,
		Path:      lockInfo.Path,
	}}
	if !lockInfo.Created.IsZero() {
		protoLock.Info.Created = timestamppb.New(lockInfo.Created)
	}
	return protoLock
}

func mapServiceError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		return connect.NewError(connect.CodeNotFound, err)
	case strings.Contains(msg, "already exists"):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case strings.Contains(msg, "invalid"), strings.Contains(msg, "required"), strings.Contains(msg, "guid"):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case strings.Contains(msg, "locked"):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case strings.Contains(msg, "cycle"), strings.Contains(msg, "conflict"):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

// Label Management Handlers

// UpdateStateLabels modifies labels on an existing state (add/remove operations).
// T035: Implements UpdateStateLabels RPC handler with validation error mapping.
func (h *StateServiceHandler) UpdateStateLabels(
	ctx context.Context,
	req *connect.Request[statev1.UpdateStateLabelsRequest],
) (*connect.Response[statev1.UpdateStateLabelsResponse], error) {
	// Convert proto LabelValue map to models.LabelMap
	adds := make(models.LabelMap)
	for key, labelValue := range req.Msg.Adds {
		adds[key] = protoLabelValueToGo(labelValue)
	}

	// Call service to update labels
	err := h.service.UpdateLabels(ctx, req.Msg.StateId, adds, req.Msg.Removals)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Fetch updated state for response
	state, err := h.service.GetStateByGUID(ctx, req.Msg.StateId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Convert labels to proto format
	protoLabels := make(map[string]*statev1.LabelValue)
	for k, v := range state.Labels {
		protoLabels[k] = goValueToProtoLabel(v)
	}

	// Get policy version if available
	policyVersion := int32(0)
	if h.policyService != nil {
		policy, err := h.policyService.GetPolicy(ctx)
		if err == nil && policy != nil {
			policyVersion = int32(policy.Version)
		}
	}

	resp := &statev1.UpdateStateLabelsResponse{
		StateId:       state.GUID,
		Labels:        protoLabels,
		PolicyVersion: policyVersion,
		UpdatedAt:     timestamppb.New(state.UpdatedAt),
	}

	return connect.NewResponse(resp), nil
}

// GetLabelPolicy retrieves the current label validation policy.
// T036: Implements GetLabelPolicy RPC handler.
func (h *StateServiceHandler) GetLabelPolicy(
	ctx context.Context,
	req *connect.Request[statev1.GetLabelPolicyRequest],
) (*connect.Response[statev1.GetLabelPolicyResponse], error) {
	if h.policyService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("policy service not configured"))
	}

	policy, err := h.policyService.GetPolicy(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Marshal policy to JSON
	policyValue, err := policy.PolicyJSON.Value()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal policy: %w", err))
	}

	policyJSON, ok := policyValue.(string)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("policy value is not string"))
	}

	resp := &statev1.GetLabelPolicyResponse{
		Version:    int32(policy.Version),
		PolicyJson: policyJSON,
		CreatedAt:  timestamppb.New(policy.CreatedAt),
		UpdatedAt:  timestamppb.New(policy.UpdatedAt),
	}

	return connect.NewResponse(resp), nil
}

// SetLabelPolicy updates the label validation policy with version increment.
// T037: Implements SetLabelPolicy RPC handler with FR-029 validation error mapping.
func (h *StateServiceHandler) SetLabelPolicy(
	ctx context.Context,
	req *connect.Request[statev1.SetLabelPolicyRequest],
) (*connect.Response[statev1.SetLabelPolicyResponse], error) {
	if h.policyService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("policy service not configured"))
	}

	// Validate and set policy (PolicyService handles validation via PolicyValidator)
	err := h.policyService.SetPolicyFromJSON(ctx, []byte(req.Msg.PolicyJson))
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Fetch updated policy for response
	policy, err := h.policyService.GetPolicy(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &statev1.SetLabelPolicyResponse{
		Version:   int32(policy.Version),
		UpdatedAt: timestamppb.New(policy.UpdatedAt),
	}

	return connect.NewResponse(resp), nil
}

// Helper functions for state filtering and label value conversion

// filterStatesByRoleScopes filters states based on the user's role label scopes.
// For each state, it checks if the state's labels match ANY of the user's role scopes.
// Platform engineers (with no label scope constraints) see all states.
// Product engineers (with env=="dev" scope) only see states with env=dev labels.
// In no-auth mode (no principal), all states are returned.
func (h *StateServiceHandler) filterStatesByRoleScopes(ctx context.Context, summaries []statepkg.StateSummary) ([]statepkg.StateSummary, error) {
	// Get principal from context
	principal, ok := auth.GetUserFromContext(ctx)
	if !ok {
		// No principal - this means auth is disabled (no-auth mode)
		// Return all states without filtering
		return summaries, nil
	}

	// If IAM service not available, return all states (backwards compatibility)
	if h.iamService == nil {
		return summaries, nil
	}

	// Get role scopes from the user's roles
	// We need to extract the role names from the Casbin role identifiers (e.g., "role:platform-engineer")
	roleScopes := make([]string, 0, len(principal.Roles))
	for _, casbinRole := range principal.Roles {
		roleName, err := auth.ExtractRoleID(casbinRole)
		if err != nil {
			continue // Skip invalid role format
		}

		role, err := h.iamService.GetRoleByName(ctx, roleName)
		if err != nil {
			continue // Skip roles that don't exist
		}

		roleScopes = append(roleScopes, role.ScopeExpr)
	}

	// If user has no roles or couldn't fetch any, return all states
	// This handles edge cases where role lookup fails
	if len(roleScopes) == 0 {
		return summaries, nil
	}

	// Filter states based on role scopes
	filtered := make([]statepkg.StateSummary, 0, len(summaries))
	for _, summary := range summaries {
		// Check if state matches ANY of the user's role scopes
		matchesAnyScope := false
		for _, scopeExpr := range roleScopes {
			// Empty scope expression means no constraint (matches all states)
			if strings.TrimSpace(scopeExpr) == "" {
				matchesAnyScope = true
				break
			}

			// Evaluate bexpr against state labels
			if auth.EvaluateBexpr(scopeExpr, summary.Labels) {
				matchesAnyScope = true
				break
			}
		}

		if matchesAnyScope {
			filtered = append(filtered, summary)
		}
	}

	return filtered, nil
}

// Helper functions for label value conversion

// protoLabelValueToGo converts proto LabelValue to Go value (string, float64, or bool).
func protoLabelValueToGo(labelValue *statev1.LabelValue) interface{} {
	switch v := labelValue.Value.(type) {
	case *statev1.LabelValue_StringValue:
		return v.StringValue
	case *statev1.LabelValue_NumberValue:
		return v.NumberValue
	case *statev1.LabelValue_BoolValue:
		return v.BoolValue
	default:
		return nil
	}
}

// goValueToProtoLabel converts Go value to proto LabelValue.
func goValueToProtoLabel(value interface{}) *statev1.LabelValue {
	switch v := value.(type) {
	case string:
		return &statev1.LabelValue{Value: &statev1.LabelValue_StringValue{StringValue: v}}
	case float64:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: v}}
	case bool:
		return &statev1.LabelValue{Value: &statev1.LabelValue_BoolValue{BoolValue: v}}
	default:
		return &statev1.LabelValue{Value: &statev1.LabelValue_StringValue{StringValue: fmt.Sprintf("%v", v)}}
	}
}
