package middleware

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
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

			procedure := req.Spec().Procedure
			obj := "" // Can be objType (like "state") or specific resource ID (for ownership checks)
			action := ""
			var labels map[string]any

			//nolint:gocritic
			switch procedure {
			// --- Static Permission Checks (no resource-specific data needed) ---
			case statev1connect.StateServiceListStatesProcedure, statev1connect.StateServiceGetStateInfoProcedure:
				obj = auth.ObjectTypeState
				action = auth.StateList
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
				labels = state.Labels
			default:
				// Deny any RPC that is not explicitly listed.
				return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("access to procedure %s is denied by default policy", procedure))
			}

			if obj == "" || action == "" {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("authorization scheme not defined for %s", procedure))
			}

			log.Printf("enforcing principal %s for action '%s' on object '%s' with labels %v", principal.PrincipalID, action, obj, labels)

			// Perform the enforcement check
			if labels == nil {
				labels = make(map[string]any)
			}
			if deps.Enforcer == nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("authorization enforcer not initialized"))
			}

			allowed, err := deps.Enforcer.Enforce(principal.PrincipalID, obj, action, labels)
			if err != nil {
				log.Printf("error enforce query for %s: %v", principal.PrincipalID, err)
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("authorization enforcement error: %w", err))
			}

			if !allowed {
				return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission denied for action '%s' on object '%s'", action, obj))
			}

			// If authorized, proceed with the request.
			return next(ctx, req)
		})
	})
}
