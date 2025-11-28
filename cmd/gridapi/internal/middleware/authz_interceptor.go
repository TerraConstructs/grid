package middleware

import (
	"context"
	"fmt"
	"log"
	"maps"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
)

// NewAuthzInterceptor creates a Connect UnaryInterceptor that enforces Casbin policies.
// It checks permissions for each RPC call, including loading resource-specific attributes
// like state labels when necessary for a policy decision.
func NewAuthzInterceptor(deps AuthzDependencies) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			principal, ok := auth.GetUserFromContext(ctx)
			if !ok {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("no authenticated principal found in context"))
			}

			// Phase 4: Convert to iam.Principal for authorization
			// Only roles are needed for authorization checks
			iamPrincipal := &iam.Principal{
				Roles: principal.Roles,
			}

			procedure := req.Spec().Procedure
			obj := "" // Can be objType (like "state") or specific resource ID (for ownership checks)
			action := ""
			var labels map[string]any

			//nolint:gocritic
			switch procedure {
			// --- Static Permission Checks (no resource-specific data needed) ---
			case statev1connect.StateServiceListStatesProcedure:
				obj = auth.ObjectTypeState
				action = auth.StateList
			case statev1connect.StateServiceListAllEdgesProcedure:
				obj = auth.ObjectTypeState
				action = auth.DependencyListAll
			case statev1connect.StateServiceGetLabelPolicyProcedure:
				obj = auth.ObjectTypePolicy
				action = auth.PolicyRead
			case statev1connect.StateServiceSetLabelPolicyProcedure:
				obj = auth.ObjectTypePolicy
				action = auth.PolicyWrite
			case statev1connect.StateServiceCreateServiceAccountProcedure, statev1connect.StateServiceListServiceAccountsProcedure, statev1connect.StateServiceRevokeServiceAccountProcedure, statev1connect.StateServiceRotateServiceAccountProcedure:
				obj = auth.ObjectTypeAdmin
				action = auth.AdminServiceAccountManage
			case statev1connect.StateServiceCreateRoleProcedure, statev1connect.StateServiceListRolesProcedure, statev1connect.StateServiceUpdateRoleProcedure, statev1connect.StateServiceDeleteRoleProcedure:
				obj = auth.ObjectTypeAdmin
				action = auth.AdminRoleManage
			case statev1connect.StateServiceAssignRoleProcedure, statev1connect.StateServiceRemoveRoleProcedure:
				obj = auth.ObjectTypeAdmin
				action = auth.AdminUserAssign
			case statev1connect.StateServiceAssignGroupRoleProcedure, statev1connect.StateServiceRemoveGroupRoleProcedure, statev1connect.StateServiceListGroupRolesProcedure:
				obj = auth.ObjectTypeAdmin
				action = auth.AdminGroupAssign
			case statev1connect.StateServiceGetEffectivePermissionsProcedure:
				// Ownership-aware check: allow users to query their own permissions
				r := req.Any().(*statev1.GetEffectivePermissionsRequest)
				targetPrincipalID := r.GetPrincipalId()
				targetPrincipalType := r.GetPrincipalType()

				if targetPrincipalID == "" {
					// Default to the caller's own ID (already prefixed)
					obj = principal.PrincipalID
				} else {
					// Request specifies a target principal - prefix it based on type
					switch targetPrincipalType {
					case "user":
						obj = auth.UserID(targetPrincipalID)
					case "service_account":
						obj = auth.ServiceAccountID(targetPrincipalID)
					default:
						return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid principal type: %s", targetPrincipalType))
					}
				}
				action = auth.ReadSelf
			case statev1connect.StateServiceListSessionsProcedure:
				// Ownership-aware check: allow users to list their own sessions
				r := req.Any().(*statev1.ListSessionsRequest)
				targetUserID := r.GetUserId()

				if targetUserID == "" {
					// Default to the caller's own ID
					// Sessions are only for users, not service accounts
					if principal.Type != auth.PrincipalTypeUser {
						return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service accounts cannot list sessions"))
					}
					obj = principal.PrincipalID // Already prefixed as user:...
				} else {
					// Request specifies a target user ID (unprefixed) - prefix it
					obj = auth.UserID(targetUserID)
				}
				action = auth.ReadSelf
			case statev1connect.StateServiceRevokeSessionProcedure:
				obj = auth.ObjectTypeAdmin
				action = auth.AdminSessionRevoke

			// --- Dynamic Permission Checks (resource-specific data required) ---
			case statev1connect.StateServiceCreateStateProcedure:
				// Extract labels from CreateStateRequest for create constraint checking
				r := req.Any().(*statev1.CreateStateRequest)
				obj = auth.ObjectTypeState
				action = auth.StateCreate
				// Convert proto labels map to labels for enforcement
				if r.Labels != nil {
					labels = make(map[string]any, len(r.Labels))
					for k, v := range r.Labels {
						labels[k] = v
					}
				}
			case statev1connect.StateServiceGetStateConfigProcedure, statev1connect.StateServiceGetStateLockProcedure, statev1connect.StateServiceUnlockStateProcedure, statev1connect.StateServiceUpdateStateLabelsProcedure:
				obj = auth.ObjectTypeState
				action = auth.StateRead // Default to read, specific handlers might override
				var stateID string
				switch r := req.Any().(type) {
				case *statev1.GetStateConfigRequest:
					// Logic ID is used here, need to resolve to GUID
					guid, _, err := deps.StateService.GetStateConfig(ctx, r.LogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found: %w", err))
					}
					stateID = guid
				case *statev1.GetStateLockRequest:
					stateID = r.Guid
				case *statev1.UnlockStateRequest:
					stateID = r.Guid
					action = auth.TfstateUnlock // Specific action
				case *statev1.UpdateStateLabelsRequest:
					stateID = r.StateId
					action = auth.StateUpdateLabels // Specific action
				default:
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("unhandled dynamic authz type for %s", procedure))
				}
				state, err := deps.StateService.GetStateByGUID(ctx, stateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)
			case statev1connect.StateServiceGetStateInfoProcedure:
				obj = auth.ObjectTypeState
				action = auth.StateRead
				var stateID string
				r := req.Any().(*statev1.GetStateInfoRequest)

				// Handle oneof state (logic_id or guid)
				switch state := r.State.(type) {
				case *statev1.GetStateInfoRequest_LogicId:
					// Resolve logic_id to GUID
					guid, _, err := deps.StateService.GetStateConfig(ctx, state.LogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found: %w", err))
					}
					stateID = guid
				case *statev1.GetStateInfoRequest_Guid:
					stateID = state.Guid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required (logic_id or guid)"))
				}

				// Load state to get labels for authorization
				state, err := deps.StateService.GetStateByGUID(ctx, stateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)
			case statev1connect.StateServiceListStateOutputsProcedure:
				obj = auth.ObjectTypeState
				action = auth.StateOutputList
				var stateID string
				r := req.Any().(*statev1.ListStateOutputsRequest)

				// Handle oneof state (logic_id or guid)
				switch state := r.State.(type) {
				case *statev1.ListStateOutputsRequest_LogicId:
					// Resolve logic_id to GUID
					guid, _, err := deps.StateService.GetStateConfig(ctx, state.LogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found: %w", err))
					}
					stateID = guid
				case *statev1.ListStateOutputsRequest_Guid:
					stateID = state.Guid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required (logic_id or guid)"))
				}

				// Load state to get labels for authorization
				state, err := deps.StateService.GetStateByGUID(ctx, stateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)

			// --- Dependency Authorization (two-check model) ---
			case statev1connect.StateServiceAddDependencyProcedure:
				// Two-check authorization: both FROM (read source) and TO (write destination) states must be accessible
				// This case performs complete authorization and returns directly without falling through to the final check
				r := req.Any().(*statev1.AddDependencyRequest)

				// Extract FROM state (producer/source)
				var fromStateID string
				switch from := r.FromState.(type) {
				case *statev1.AddDependencyRequest_FromLogicId:
					guid, _, err := deps.StateService.GetStateConfig(ctx, from.FromLogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("from state not found: %w", err))
					}
					fromStateID = guid
				case *statev1.AddDependencyRequest_FromGuid:
					fromStateID = from.FromGuid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("from state reference required"))
				}

				// Extract TO state (consumer/destination)
				var toStateID string
				switch to := r.ToState.(type) {
				case *statev1.AddDependencyRequest_ToLogicId:
					guid, _, err := deps.StateService.GetStateConfig(ctx, to.ToLogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("to state not found: %w", err))
					}
					toStateID = guid
				case *statev1.AddDependencyRequest_ToGuid:
					toStateID = to.ToGuid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("to state reference required"))
				}

				// Check 1: User must have permission to read outputs from FROM state (producer)
				fromState, err := deps.StateService.GetStateByGUID(ctx, fromStateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("from state not found: %w", err))
				}
				// Convert LabelMap to map[string]any for authorization
				fromLabels := make(map[string]any, len(fromState.Labels))
				maps.Copy(fromLabels, fromState.Labels)
				log.Printf("enforcing principal %s for action '%s' on object '%s' with labels %v", principal.PrincipalID, auth.StateOutputRead, auth.ObjectTypeState, fromLabels)
				// Phase 4: Use IAM service for read-only authorization
				allowed, err := deps.IAMService.Authorize(ctx, iamPrincipal, auth.ObjectTypeState, auth.StateOutputRead, fromLabels)
				if err != nil {
					log.Printf("error enforcing from state auth for %s: %v", principal.PrincipalID, err)
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("authorization error: %w", err))
				}
				if !allowed {
					return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission denied: cannot read outputs from source state"))
				}

				// Check 2: User must have permission to create dependency on TO state (consumer)
				toState, err := deps.StateService.GetStateByGUID(ctx, toStateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("to state not found: %w", err))
				}
				// Convert LabelMap to map[string]any for authorization
				toLabels := make(map[string]any, len(toState.Labels))
				maps.Copy(toLabels, toState.Labels)
				log.Printf("enforcing principal %s for action '%s' on object '%s' with labels %v", principal.PrincipalID, auth.DependencyCreate, auth.ObjectTypeState, toLabels)
				// Phase 4: Use IAM service for read-only authorization
				allowed, err = deps.IAMService.Authorize(ctx, iamPrincipal, auth.ObjectTypeState, auth.DependencyCreate, toLabels)
				if err != nil {
					log.Printf("error enforcing to state auth for %s: %v", principal.PrincipalID, err)
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("authorization error: %w", err))
				}
				if !allowed {
					return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission denied: cannot create dependency on destination state"))
				}

				// Both checks passed. Authorization complete. Proceed directly to the RPC handler.
				return next(ctx, req)

			case statev1connect.StateServiceListDependenciesProcedure:
				obj = auth.ObjectTypeState
				action = auth.DependencyList
				var stateID string
				r := req.Any().(*statev1.ListDependenciesRequest)

				// Handle oneof state (logic_id or guid)
				switch state := r.State.(type) {
				case *statev1.ListDependenciesRequest_LogicId:
					guid, _, err := deps.StateService.GetStateConfig(ctx, state.LogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found: %w", err))
					}
					stateID = guid
				case *statev1.ListDependenciesRequest_Guid:
					stateID = state.Guid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required"))
				}

				state, err := deps.StateService.GetStateByGUID(ctx, stateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)

			case statev1connect.StateServiceListDependentsProcedure:
				obj = auth.ObjectTypeState
				action = auth.DependencyList
				var stateID string
				r := req.Any().(*statev1.ListDependentsRequest)

				// Handle oneof state (logic_id or guid)
				switch state := r.State.(type) {
				case *statev1.ListDependentsRequest_LogicId:
					guid, _, err := deps.StateService.GetStateConfig(ctx, state.LogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found: %w", err))
					}
					stateID = guid
				case *statev1.ListDependentsRequest_Guid:
					stateID = state.Guid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required"))
				}

				state, err := deps.StateService.GetStateByGUID(ctx, stateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)

			case statev1connect.StateServiceGetDependencyGraphProcedure:
				obj = auth.ObjectTypeState
				action = auth.DependencyList
				var stateID string
				r := req.Any().(*statev1.GetDependencyGraphRequest)

				// Handle oneof state (logic_id or guid)
				switch state := r.State.(type) {
				case *statev1.GetDependencyGraphRequest_LogicId:
					guid, _, err := deps.StateService.GetStateConfig(ctx, state.LogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found: %w", err))
					}
					stateID = guid
				case *statev1.GetDependencyGraphRequest_Guid:
					stateID = state.Guid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required"))
				}

				state, err := deps.StateService.GetStateByGUID(ctx, stateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)

			case statev1connect.StateServiceRemoveDependencyProcedure:
				obj = auth.ObjectTypeState
				action = auth.DependencyDelete

				r := req.Any().(*statev1.RemoveDependencyRequest)

				// Load edge to get destination state GUID
				edge, err := deps.StateService.GetEdgeByID(ctx, r.EdgeId)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("edge not found: %w", err))
				}

				// Load destination state to get labels
				state, err := deps.StateService.GetStateByGUID(ctx, edge.ToState)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("destination state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)

			// --- Output Schema Management ---
			case statev1connect.StateServiceSetOutputSchemaProcedure:
				obj = auth.ObjectTypeState
				action = auth.StateOutputSchemaWrite
				var stateID string
				r := req.Any().(*statev1.SetOutputSchemaRequest)

				// Handle oneof state (logic_id or guid)
				switch state := r.State.(type) {
				case *statev1.SetOutputSchemaRequest_StateLogicId:
					// Resolve logic_id to GUID
					guid, _, err := deps.StateService.GetStateConfig(ctx, state.StateLogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found: %w", err))
					}
					stateID = guid
				case *statev1.SetOutputSchemaRequest_StateGuid:
					stateID = state.StateGuid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required (state_logic_id or state_guid)"))
				}

				// Load state to get labels for authorization
				state, err := deps.StateService.GetStateByGUID(ctx, stateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)

			case statev1connect.StateServiceGetOutputSchemaProcedure:
				obj = auth.ObjectTypeState
				action = auth.StateOutputSchemaRead
				var stateID string
				r := req.Any().(*statev1.GetOutputSchemaRequest)

				// Handle oneof state (logic_id or guid)
				switch state := r.State.(type) {
				case *statev1.GetOutputSchemaRequest_StateLogicId:
					// Resolve logic_id to GUID
					guid, _, err := deps.StateService.GetStateConfig(ctx, state.StateLogicId)
					if err != nil {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found: %w", err))
					}
					stateID = guid
				case *statev1.GetOutputSchemaRequest_StateGuid:
					stateID = state.StateGuid
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required (state_logic_id or state_guid)"))
				}

				// Load state to get labels for authorization
				state, err := deps.StateService.GetStateByGUID(ctx, stateID)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("state not found for authz: %w", err))
				}
				labels = make(map[string]any, len(state.Labels))
				maps.Copy(labels, state.Labels)

			default:
				// Deny any RPC that is not explicitly listed.
				return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("access to procedure %s is denied by default policy", procedure))
			}

			if obj == "" || action == "" {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("authorization scheme not defined for %s", procedure))
			}

			log.Printf("enforcing principal %s for action '%s' on object '%s' with labels %v", principal.PrincipalID, action, obj, labels)

			// Phase 4: Perform authorization check using IAM service (read-only, no Casbin mutation)
			if labels == nil {
				labels = make(map[string]any)
			}
			if deps.IAMService == nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("IAM service not initialized"))
			}

			allowed, err := deps.IAMService.Authorize(ctx, iamPrincipal, obj, action, labels)
			if err != nil {
				log.Printf("error enforce query for %s: %v", principal.PrincipalID, err)
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("authorization enforcement error: %w", err))
			}

			if !allowed {
				log.Printf("authorization denied: principal %s (roles=%v) for %s on %s (labels=%v)",
					principal.PrincipalID, principal.Roles, action, obj, labels)
				return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission denied for action '%s' on object '%s'", action, obj))
			}

			// If authorized, proceed with the request.
			return next(ctx, req)
		})
	})
}
