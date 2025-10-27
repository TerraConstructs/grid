package sdk

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
)

// Client provides a high-level interface to the Grid state management API.
// It wraps the generated Connect RPC client with ergonomic methods.
type Client struct {
	rpc     statev1connect.StateServiceClient
	baseURL string
}

// ListStatesOptions configures optional parameters for ListStatesWithOptions.
type ListStatesOptions struct {
	Filter        string
	IncludeLabels *bool
}

// UpdateStateLabelsInput describes label mutations for UpdateStateLabels.
type UpdateStateLabelsInput struct {
	StateID  string
	Adds     LabelMap
	Removals []string
}

// ClientOptions configures SDK client construction.
type ClientOptions struct {
	HTTPClient *http.Client
}

// ClientOption mutates ClientOptions.
type ClientOption func(*ClientOptions)

// WithHTTPClient overrides the HTTP client used for RPC calls.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(opts *ClientOptions) {
		opts.HTTPClient = client
	}
}

// NewClient creates a new Grid SDK client that communicates with the API server at baseURL.
// An http.Client is created automatically when one is not supplied.
func NewClient(baseURL string, optFns ...ClientOption) *Client {
	opts := ClientOptions{}
	for _, fn := range optFns {
		fn(&opts)
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}

	rpcClient := statev1connect.NewStateServiceClient(opts.HTTPClient, baseURL)

	return &Client{
		rpc:     rpcClient,
		baseURL: baseURL,
	}
}

// CreateState creates a new Terraform state. When GUID is empty, a random UUID is generated.
func (c *Client) CreateState(ctx context.Context, input CreateStateInput) (*State, error) {
	if input.LogicID == "" {
		return nil, fmt.Errorf("logic ID is required")
	}

	guid := input.GUID
	if guid == "" {
		guid = uuid.NewString()
	}

	// Convert LabelMap (map[string]any) to proto labels (map[string]string)
	protoLabels := make(map[string]string)
	for k, v := range input.Labels {
		if strVal, ok := v.(string); ok {
			protoLabels[k] = strVal
		}
	}

	req := connect.NewRequest(&statev1.CreateStateRequest{
		Guid:    guid,
		LogicId: input.LogicID,
		Labels:  protoLabels,
	})

	resp, err := c.rpc.CreateState(ctx, req)
	if err != nil {
		return nil, err
	}

	return &State{
		GUID:          resp.Msg.GetGuid(),
		LogicID:       resp.Msg.GetLogicId(),
		BackendConfig: backendConfigFromProto(resp.Msg.BackendConfig),
	}, nil
}

// ListStates returns summary information for all states managed by the server.
func (c *Client) ListStates(ctx context.Context) ([]StateSummary, error) {
	return c.ListStatesWithOptions(ctx, ListStatesOptions{})
}

// ListStatesWithOptions returns summaries with optional filter/label projection controls.
func (c *Client) ListStatesWithOptions(ctx context.Context, opts ListStatesOptions) ([]StateSummary, error) {
	req := connect.NewRequest(&statev1.ListStatesRequest{})
	if opts.Filter != "" {
		req.Msg.Filter = &opts.Filter
	}
	if opts.IncludeLabels != nil {
		req.Msg.IncludeLabels = opts.IncludeLabels
	}

	resp, err := c.rpc.ListStates(ctx, req)
	if err != nil {
		return nil, err
	}

	summaries := make([]StateSummary, 0, len(resp.Msg.GetStates()))
	for _, info := range resp.Msg.GetStates() {
		summaries = append(summaries, stateSummaryFromProto(info))
	}
	return summaries, nil
}

// GetState retrieves state metadata and backend configuration using either GUID or logic ID.
func (c *Client) GetState(ctx context.Context, ref StateReference) (*State, error) {
	if ref.LogicID != "" {
		cfg, err := c.rpc.GetStateConfig(ctx, connect.NewRequest(&statev1.GetStateConfigRequest{LogicId: ref.LogicID}))
		if err != nil {
			return nil, err
		}
		return &State{
			GUID:          cfg.Msg.GetGuid(),
			LogicID:       ref.LogicID,
			BackendConfig: backendConfigFromProto(cfg.Msg.BackendConfig),
		}, nil
	}

	if ref.GUID == "" {
		return nil, fmt.Errorf("state reference requires guid or logic ID")
	}

	// No direct RPC by GUID; list and locate the desired state, then fetch config by logic ID.
	summaries, err := c.ListStates(ctx)
	if err != nil {
		return nil, err
	}
	for _, summary := range summaries {
		if summary.GUID == ref.GUID {
			cfg, err := c.rpc.GetStateConfig(ctx, connect.NewRequest(&statev1.GetStateConfigRequest{LogicId: summary.LogicID}))
			if err != nil {
				return nil, err
			}
			return &State{
				GUID:          cfg.Msg.GetGuid(),
				LogicID:       summary.LogicID,
				BackendConfig: backendConfigFromProto(cfg.Msg.BackendConfig),
			}, nil
		}
	}

	return nil, fmt.Errorf("state with guid %s not found", ref.GUID)
}

// GetStateLock inspects the current lock status and metadata for a state by its GUID.
func (c *Client) GetStateLock(ctx context.Context, guid string) (StateLock, error) {
	req := connect.NewRequest(&statev1.GetStateLockRequest{Guid: guid})

	resp, err := c.rpc.GetStateLock(ctx, req)
	if err != nil {
		return StateLock{}, err
	}

	return stateLockFromProto(resp.Msg.GetLock()), nil
}

// UnlockState releases a lock on a state identified by GUID.
// The lockID must match the ID of the current lock, or the operation will fail.
func (c *Client) UnlockState(ctx context.Context, guid, lockID string) (StateLock, error) {
	req := connect.NewRequest(&statev1.UnlockStateRequest{
		Guid:   guid,
		LockId: lockID,
	})

	resp, err := c.rpc.UnlockState(ctx, req)
	if err != nil {
		return StateLock{}, err
	}

	return stateLockFromProto(resp.Msg.GetLock()), nil
}

// ListStateOutputs returns the output keys from a state's Terraform JSON.
// Output values are not returned for security/size reasons - only keys and sensitive flags.
func (c *Client) ListStateOutputs(ctx context.Context, ref StateReference) ([]OutputKey, error) {
	if ref.LogicID == "" && ref.GUID == "" {
		return nil, fmt.Errorf("state reference requires guid or logic ID")
	}

	req := connect.NewRequest(&statev1.ListStateOutputsRequest{})
	if ref.LogicID != "" {
		req.Msg.State = &statev1.ListStateOutputsRequest_LogicId{LogicId: ref.LogicID}
	} else {
		req.Msg.State = &statev1.ListStateOutputsRequest_Guid{Guid: ref.GUID}
	}

	resp, err := c.rpc.ListStateOutputs(ctx, req)
	if err != nil {
		return nil, err
	}

	return outputKeysFromProto(resp.Msg.GetOutputs()), nil
}

// UpdateStateLabels mutates labels on an existing state and returns the updated set.
func (c *Client) UpdateStateLabels(ctx context.Context, input UpdateStateLabelsInput) (*UpdateStateLabelsResult, error) {
	if input.StateID == "" {
		return nil, fmt.Errorf("state ID is required")
	}

	req := connect.NewRequest(&statev1.UpdateStateLabelsRequest{
		StateId:  input.StateID,
		Adds:     ConvertLabelsToProto(input.Adds),
		Removals: append([]string(nil), input.Removals...),
	})

	resp, err := c.rpc.UpdateStateLabels(ctx, req)
	if err != nil {
		return nil, err
	}

	result := &UpdateStateLabelsResult{
		StateID:       resp.Msg.GetStateId(),
		Labels:        ConvertProtoLabels(resp.Msg.GetLabels()),
		PolicyVersion: resp.Msg.GetPolicyVersion(),
	}
	if resp.Msg.UpdatedAt != nil {
		result.UpdatedAt = resp.Msg.UpdatedAt.AsTime()
	}
	return result, nil
}

// GetLabelPolicy retrieves the current label validation policy.
func (c *Client) GetLabelPolicy(ctx context.Context) (*LabelPolicy, error) {
	resp, err := c.rpc.GetLabelPolicy(ctx, connect.NewRequest(&statev1.GetLabelPolicyRequest{}))
	if err != nil {
		return nil, err
	}
	return labelPolicyFromProto(resp.Msg), nil
}

// SetLabelPolicy replaces the label policy using the provided JSON definition.
func (c *Client) SetLabelPolicy(ctx context.Context, policyJSON []byte) (*LabelPolicy, error) {
	req := connect.NewRequest(&statev1.SetLabelPolicyRequest{PolicyJson: string(policyJSON)})
	resp, err := c.rpc.SetLabelPolicy(ctx, req)
	if err != nil {
		return nil, err
	}

	policy := &LabelPolicy{
		Version:    resp.Msg.GetVersion(),
		PolicyJSON: string(policyJSON),
	}
	if resp.Msg.UpdatedAt != nil {
		policy.UpdatedAt = resp.Msg.UpdatedAt.AsTime()
	}
	return policy, nil
}

// GetStateInfo retrieves comprehensive state information including dependencies, dependents, and outputs.
// This consolidates information that would otherwise require multiple RPC calls.
func (c *Client) GetStateInfo(ctx context.Context, ref StateReference) (*StateInfo, error) {
	if ref.LogicID == "" && ref.GUID == "" {
		return nil, fmt.Errorf("state reference requires guid or logic ID")
	}

	req := connect.NewRequest(&statev1.GetStateInfoRequest{})
	if ref.LogicID != "" {
		req.Msg.State = &statev1.GetStateInfoRequest_LogicId{LogicId: ref.LogicID}
	} else {
		req.Msg.State = &statev1.GetStateInfoRequest_Guid{Guid: ref.GUID}
	}

	resp, err := c.rpc.GetStateInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	msg := resp.Msg

	// Convert dependencies
	dependencies := make([]DependencyEdge, 0, len(msg.GetDependencies()))
	for _, dep := range msg.GetDependencies() {
		dependencies = append(dependencies, dependencyEdgeFromProto(dep))
	}

	// Convert dependents
	dependents := make([]DependencyEdge, 0, len(msg.GetDependents()))
	for _, dep := range msg.GetDependents() {
		dependents = append(dependents, dependencyEdgeFromProto(dep))
	}

	stateInfo := &StateInfo{
		State: StateReference{
			GUID:    msg.GetGuid(),
			LogicID: msg.GetLogicId(),
		},
		BackendConfig: backendConfigFromProto(msg.GetBackendConfig()),
		Dependencies:  dependencies,
		Dependents:    dependents,
		Outputs:       outputKeysFromProto(msg.GetOutputs()),
		SizeBytes:     msg.GetSizeBytes(),
		Labels:        ConvertProtoLabels(msg.GetLabels()),
	}

	if msg.CreatedAt != nil {
		stateInfo.CreatedAt = msg.CreatedAt.AsTime()
	}
	if msg.UpdatedAt != nil {
		stateInfo.UpdatedAt = msg.UpdatedAt.AsTime()
	}

	return stateInfo, nil
}

// GetEffectivePermissions retrieves the effective permissions for a principal.
// The principal can be identified by prefixing the ID ("user:alice", "sa:deployer")
// or by explicitly setting PrincipalType. Valid types: "user", "service_account".
func (c *Client) GetEffectivePermissions(ctx context.Context, input GetEffectivePermissionsInput) (*GetEffectivePermissionsResult, error) {
	var principalId string
	var principalType string
	var hasPrefix bool

	if principalId, hasPrefix = strings.CutPrefix(input.PrincipalID, "sa:"); hasPrefix {
		principalType = "service_account"
	} else if principalId, hasPrefix = strings.CutPrefix(input.PrincipalID, "user:"); hasPrefix {
		principalType = "user"
	} else {
		principalId = input.PrincipalID
	}

	if !hasPrefix {
		if input.PrincipalType == "" {
			return nil, fmt.Errorf("principal ID must be prefixed with 'user:' or 'sa:', or principal type must be specified")
		}
		principalType = input.PrincipalType
	}

	// Validate consistency if both prefix and explicit type provided
	if hasPrefix && input.PrincipalType != "" && input.PrincipalType != principalType {
		return nil, fmt.Errorf("principal ID prefix '%s:' does not match specified principal type '%s'", principalType, input.PrincipalType)
	}

	req := connect.NewRequest(&statev1.GetEffectivePermissionsRequest{
		PrincipalType: principalType,
		PrincipalId:   principalId,
	})

	resp, err := c.rpc.GetEffectivePermissions(ctx, req)
	if err != nil {
		return nil, err
	}

	return &GetEffectivePermissionsResult{
		Permissions: &EffectivePermissions{
			Roles:                      resp.Msg.Permissions.Roles,
			Actions:                    resp.Msg.Permissions.Actions,
			LabelScopeExprs:            resp.Msg.Permissions.LabelScopeExprs,
			EffectiveCreateConstraints: createConstraintsFromProto(resp.Msg.Permissions.EffectiveCreateConstraints),
			EffectiveImmutableKeys:     resp.Msg.Permissions.EffectiveImmutableKeys,
		},
	}, nil
}

// AssignGroupRole assigns a group to a role.
func (c *Client) AssignGroupRole(ctx context.Context, input AssignGroupRoleInput) (*AssignGroupRoleResult, error) {
	req := connect.NewRequest(&statev1.AssignGroupRoleRequest{
		GroupName: input.GroupName,
		RoleName:  input.RoleName,
	})

	resp, err := c.rpc.AssignGroupRole(ctx, req)
	if err != nil {
		return nil, err
	}

	return &AssignGroupRoleResult{
		Success:    resp.Msg.Success,
		AssignedAt: resp.Msg.AssignedAt.AsTime(),
	}, nil
}

// RemoveGroupRole removes a group from a role.
func (c *Client) RemoveGroupRole(ctx context.Context, input RemoveGroupRoleInput) (*RemoveGroupRoleResult, error) {
	req := connect.NewRequest(&statev1.RemoveGroupRoleRequest{
		GroupName: input.GroupName,
		RoleName:  input.RoleName,
	})

	resp, err := c.rpc.RemoveGroupRole(ctx, req)
	if err != nil {
		return nil, err
	}

	return &RemoveGroupRoleResult{
		Success: resp.Msg.Success,
	}, nil
}

// ListGroupRoles lists group-to-role assignments.

func (c *Client) ListGroupRoles(ctx context.Context, input ListGroupRolesInput) (*ListGroupRolesResult, error) {

	return nil, fmt.Errorf("not implemented")

}

// ExportRoles exports roles to a JSON string.
func (c *Client) ExportRoles(ctx context.Context, input ExportRolesInput) (*ExportRolesResult, error) {
	return nil, fmt.Errorf("not implemented")
}

// // ExportRoles exports roles to a JSON string.
// func (c *Client) ExportRoles(ctx context.Context, input ExportRolesInput) (*ExportRolesResult, error) {
// 	req := connect.NewRequest(&statev1.ExportRolesRequest{
// 		RoleNames: input.RoleNames,
// 	})

// 	resp, err := c.rpc.ExportRoles(ctx, req)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &ExportRolesResult{
// 		RolesJSON: resp.Msg.RolesJson,
// 	}, nil
// }

// ImportRoles imports roles from a JSON string.
func (c *Client) ImportRoles(ctx context.Context, input ImportRolesInput) (*ImportRolesResult, error) {
	return nil, fmt.Errorf("not implemented")
}

// // ImportRoles imports roles from a JSON string.
// func (c *Client) ImportRoles(ctx context.Context, input ImportRolesInput) (*ImportRolesResult, error) {
// 	req := connect.NewRequest(&statev1.ImportRolesRequest{
// 		RolesJson: input.RolesJSON,
// 		Force:     input.Force,
// 	})

// 	resp, err := c.rpc.ImportRoles(ctx, req)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &ImportRolesResult{
// 		ImportedCount: int(resp.Msg.ImportedCount),
// 		SkippedCount:  int(resp.Msg.SkippedCount),
// 		Errors:        resp.Msg.Errors,
// 	}, nil
// }
