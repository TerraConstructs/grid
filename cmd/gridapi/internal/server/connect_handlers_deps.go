package server

import (
	"context"
	"fmt"
	"log"
	"strings"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/dependency"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Dependency RPC Handlers

func (h *StateServiceHandler) AddDependency(
	ctx context.Context,
	req *connect.Request[statev1.AddDependencyRequest],
) (*connect.Response[statev1.AddDependencyResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	// Extract state references from oneof fields
	var fromLogicID, fromGUID, toLogicID, toGUID string
	if from, ok := req.Msg.FromState.(*statev1.AddDependencyRequest_FromLogicId); ok {
		fromLogicID = from.FromLogicId
	} else if from, ok := req.Msg.FromState.(*statev1.AddDependencyRequest_FromGuid); ok {
		fromGUID = from.FromGuid
	}

	if to, ok := req.Msg.ToState.(*statev1.AddDependencyRequest_ToLogicId); ok {
		toLogicID = to.ToLogicId
	} else if to, ok := req.Msg.ToState.(*statev1.AddDependencyRequest_ToGuid); ok {
		toGUID = to.ToGuid
	}

	// Build service request
	svcReq := &dependency.AddDependencyRequest{
		FromLogicID: fromLogicID,
		FromGUID:    fromGUID,
		FromOutput:  req.Msg.FromOutput,
		ToLogicID:   toLogicID,
		ToGUID:      toGUID,
	}

	if req.Msg.ToInputName != nil {
		svcReq.ToInputName = *req.Msg.ToInputName
	}
	if req.Msg.MockValueJson != nil {
		svcReq.MockValueJSON = *req.Msg.MockValueJson
	}

	edge, alreadyExists, err := h.depService.AddDependency(ctx, svcReq)
	if err != nil {
		return nil, mapServiceError(err)
	}

	protoEdge, err := h.edgeToProto(ctx, edge, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &statev1.AddDependencyResponse{
		Edge:          protoEdge,
		AlreadyExists: alreadyExists,
	}

	return connect.NewResponse(resp), nil
}

func (h *StateServiceHandler) RemoveDependency(
	ctx context.Context,
	req *connect.Request[statev1.RemoveDependencyRequest],
) (*connect.Response[statev1.RemoveDependencyResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	if err := h.depService.RemoveDependency(ctx, req.Msg.EdgeId); err != nil {
		return nil, mapServiceError(err)
	}

	return connect.NewResponse(&statev1.RemoveDependencyResponse{Success: true}), nil
}

func (h *StateServiceHandler) ListDependencies(
	ctx context.Context,
	req *connect.Request[statev1.ListDependenciesRequest],
) (*connect.Response[statev1.ListDependenciesResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	var logicID, guid string
	if state, ok := req.Msg.State.(*statev1.ListDependenciesRequest_LogicId); ok {
		logicID = state.LogicId
	} else if state, ok := req.Msg.State.(*statev1.ListDependenciesRequest_Guid); ok {
		guid = state.Guid
	}

	edges, err := h.depService.ListDependencies(ctx, logicID, guid)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Filter edges based on user's role scopes
	// Users only see edges where they can view both source and destination states
	filteredEdges, err := h.filterEdgesByRoleScopes(ctx, edges)
	if err != nil {
		return nil, mapServiceError(err)
	}

	protoEdges, err := h.edgesToProto(ctx, filteredEdges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&statev1.ListDependenciesResponse{Edges: protoEdges}), nil
}

func (h *StateServiceHandler) ListDependents(
	ctx context.Context,
	req *connect.Request[statev1.ListDependentsRequest],
) (*connect.Response[statev1.ListDependentsResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	var logicID, guid string
	if state, ok := req.Msg.State.(*statev1.ListDependentsRequest_LogicId); ok {
		logicID = state.LogicId
	} else if state, ok := req.Msg.State.(*statev1.ListDependentsRequest_Guid); ok {
		guid = state.Guid
	}

	edges, err := h.depService.ListDependents(ctx, logicID, guid)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Filter edges based on user's role scopes
	// Users only see edges where they can view both source and destination states
	filteredEdges, err := h.filterEdgesByRoleScopes(ctx, edges)
	if err != nil {
		return nil, mapServiceError(err)
	}

	protoEdges, err := h.edgesToProto(ctx, filteredEdges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&statev1.ListDependentsResponse{Edges: protoEdges}), nil
}

func (h *StateServiceHandler) SearchByOutput(
	ctx context.Context,
	req *connect.Request[statev1.SearchByOutputRequest],
) (*connect.Response[statev1.SearchByOutputResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	edges, err := h.depService.SearchByOutput(ctx, req.Msg.OutputKey)
	if err != nil {
		return nil, mapServiceError(err)
	}

	protoEdges, err := h.edgesToProto(ctx, edges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&statev1.SearchByOutputResponse{Edges: protoEdges}), nil
}

func (h *StateServiceHandler) GetTopologicalOrder(
	ctx context.Context,
	req *connect.Request[statev1.GetTopologicalOrderRequest],
) (*connect.Response[statev1.GetTopologicalOrderResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	var logicID, guid string
	if state, ok := req.Msg.State.(*statev1.GetTopologicalOrderRequest_LogicId); ok {
		logicID = state.LogicId
	} else if state, ok := req.Msg.State.(*statev1.GetTopologicalOrderRequest_Guid); ok {
		guid = state.Guid
	}

	direction := "downstream" // default
	if req.Msg.Direction != nil {
		direction = *req.Msg.Direction
	}

	layers, err := h.depService.GetTopologicalOrder(ctx, logicID, guid, direction)
	if err != nil {
		return nil, mapServiceError(err)
	}

	protoLayers := make([]*statev1.Layer, 0, len(layers))
	for _, layer := range layers {
		stateRefs := make([]*statev1.StateRef, 0, len(layer.States))
		for _, guid := range layer.States {
			state, _ := h.service.GetStateByGUID(ctx, guid)
			stateRef := &statev1.StateRef{Guid: guid}
			if state != nil {
				stateRef.LogicId = state.LogicID
			}
			stateRefs = append(stateRefs, stateRef)
		}
		protoLayers = append(protoLayers, &statev1.Layer{
			Level:  int32(layer.Level),
			States: stateRefs,
		})
	}

	return connect.NewResponse(&statev1.GetTopologicalOrderResponse{Layers: protoLayers}), nil
}

func (h *StateServiceHandler) GetStateStatus(
	ctx context.Context,
	req *connect.Request[statev1.GetStateStatusRequest],
) (*connect.Response[statev1.GetStateStatusResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	var logicID, guid string
	if state, ok := req.Msg.State.(*statev1.GetStateStatusRequest_LogicId); ok {
		logicID = state.LogicId
	} else if state, ok := req.Msg.State.(*statev1.GetStateStatusRequest_Guid); ok {
		guid = state.Guid
	}

	status, err := h.depService.GetStateStatus(ctx, logicID, guid)
	if err != nil {
		return nil, mapServiceError(err)
	}

	protoIncoming := make([]*statev1.IncomingEdgeView, 0, len(status.Incoming))
	for _, inc := range status.Incoming {
		view := &statev1.IncomingEdgeView{
			EdgeId:      inc.EdgeID,
			FromGuid:    inc.FromGUID,
			FromLogicId: inc.FromLogicID,
			FromOutput:  inc.FromOutput,
			Status:      inc.Status,
		}
		if inc.InDigest != "" {
			view.InDigest = &inc.InDigest
		}
		if inc.OutDigest != "" {
			view.OutDigest = &inc.OutDigest
		}
		if inc.LastInAt != nil {
			view.LastInAt = timestamppb.New(*inc.LastInAt)
		}
		if inc.LastOutAt != nil {
			view.LastOutAt = timestamppb.New(*inc.LastOutAt)
		}
		protoIncoming = append(protoIncoming, view)
	}

	resp := &statev1.GetStateStatusResponse{
		Guid:     status.StateGUID,
		LogicId:  status.LogicID,
		Status:   status.Status,
		Incoming: protoIncoming,
		Summary: &statev1.StatusSummary{
			IncomingClean:   int32(status.Summary.IncomingClean),
			IncomingDirty:   int32(status.Summary.IncomingDirty),
			IncomingPending: int32(status.Summary.IncomingPending),
			IncomingUnknown: int32(status.Summary.IncomingUnknown),
		},
	}

	return connect.NewResponse(resp), nil
}

func (h *StateServiceHandler) GetDependencyGraph(
	ctx context.Context,
	req *connect.Request[statev1.GetDependencyGraphRequest],
) (*connect.Response[statev1.GetDependencyGraphResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	var logicID, guid string
	if state, ok := req.Msg.State.(*statev1.GetDependencyGraphRequest_LogicId); ok {
		logicID = state.LogicId
	} else if state, ok := req.Msg.State.(*statev1.GetDependencyGraphRequest_Guid); ok {
		guid = state.Guid
	}

	graph, err := h.depService.GetDependencyGraph(ctx, logicID, guid)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Convert producers - compute backend configs inline (avoids N+1 GetStateConfig calls)
	protoProducers := make([]*statev1.ProducerState, 0, len(graph.Producers))
	for _, producer := range graph.Producers {
		// Compute backend URLs directly using GUID (avoids extra DB query per producer)
		protoProducers = append(protoProducers, &statev1.ProducerState{
			Guid:    producer.GUID,
			LogicId: producer.LogicID,
			BackendConfig: &statev1.BackendConfig{
				Address:       fmt.Sprintf("%s/tfstate/%s", h.cfg.ServerURL, producer.GUID),
				LockAddress:   fmt.Sprintf("%s/tfstate/%s/lock", h.cfg.ServerURL, producer.GUID),
				UnlockAddress: fmt.Sprintf("%s/tfstate/%s/unlock", h.cfg.ServerURL, producer.GUID),
			},
		})
	}

	// Convert edges
	protoEdges, err := h.edgesToProto(ctx, graph.Edges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &statev1.GetDependencyGraphResponse{
		ConsumerGuid:    graph.ConsumerGUID,
		ConsumerLogicId: graph.ConsumerLogicID,
		Producers:       protoProducers,
		Edges:           protoEdges,
	}

	return connect.NewResponse(resp), nil
}

// Helper function to convert edges to proto
func (h *StateServiceHandler) edgesToProto(ctx context.Context, edges []models.Edge) ([]*statev1.DependencyEdge, error) {
	if len(edges) == 0 {
		return []*statev1.DependencyEdge{}, nil
	}

	// Collect all unique GUIDs from edges (both from_state and to_state)
	guidSet := make(map[string]struct{})
	for i := range edges {
		guidSet[edges[i].FromState] = struct{}{}
		guidSet[edges[i].ToState] = struct{}{}
	}

	// Batch fetch all states at once (avoids N+1 queries)
	guids := make([]string, 0, len(guidSet))
	for guid := range guidSet {
		guids = append(guids, guid)
	}

	stateMap, err := h.service.GetStatesByGUIDs(ctx, guids)
	if err != nil {
		return nil, fmt.Errorf("batch fetch states for edges: %w", err)
	}

	// Convert edges using pre-fetched states
	protoEdges := make([]*statev1.DependencyEdge, 0, len(edges))
	for i := range edges {
		protoEdge, err := h.edgeToProtoWithCache(ctx, &edges[i], stateMap)
		if err != nil {
			return nil, err
		}
		protoEdges = append(protoEdges, protoEdge)
	}

	return protoEdges, nil
}

func (h *StateServiceHandler) edgeToProtoWithCache(ctx context.Context, edge *models.Edge, cache map[string]*models.State) (*statev1.DependencyEdge, error) {
	if cache == nil {
		cache = make(map[string]*models.State)
	}

	fromState := cache[edge.FromState]
	if fromState == nil {
		return nil, fmt.Errorf("from_state %s not found in cache", edge.FromState)
	}

	toState := cache[edge.ToState]
	if toState == nil {
		return nil, fmt.Errorf("to_state %s not found in cache", edge.ToState)
	}

	protoEdge := &statev1.DependencyEdge{
		Id:         edge.ID,
		FromGuid:   edge.FromState,
		FromOutput: edge.FromOutput,
		ToGuid:     edge.ToState,
		Status:     string(edge.Status),
	}

	if edge.ToInputName != "" {
		protoEdge.ToInputName = &edge.ToInputName
	}

	if fromState != nil {
		protoEdge.FromLogicId = fromState.LogicID
	}
	if toState != nil {
		protoEdge.ToLogicId = toState.LogicID
	}

	if edge.InDigest != "" {
		protoEdge.InDigest = &edge.InDigest
	}
	if edge.OutDigest != "" {
		protoEdge.OutDigest = &edge.OutDigest
	}
	if edge.LastInAt != nil {
		protoEdge.LastInAt = timestamppb.New(*edge.LastInAt)
	}
	if edge.LastOutAt != nil {
		protoEdge.LastOutAt = timestamppb.New(*edge.LastOutAt)
	}
	if len(edge.MockValue) > 0 {
		mockJSON := string(edge.MockValue)
		protoEdge.MockValueJson = &mockJSON
	}
	if !edge.CreatedAt.IsZero() {
		protoEdge.CreatedAt = timestamppb.New(edge.CreatedAt)
	}
	if !edge.UpdatedAt.IsZero() {
		protoEdge.UpdatedAt = timestamppb.New(edge.UpdatedAt)
	}

	return protoEdge, nil
}

// edgeToProto converts a single edge to proto (legacy wrapper for compatibility)
// For batch operations, use edgesToProto instead which avoids N+1 queries
func (h *StateServiceHandler) edgeToProto(ctx context.Context, edge *models.Edge, cache map[string]*models.State) (*statev1.DependencyEdge, error) {
	// If cache is provided and populated, use it
	if len(cache) > 0 {
		return h.edgeToProtoWithCache(ctx, edge, cache)
	}

	// Otherwise, fetch the states we need
	guids := []string{edge.FromState, edge.ToState}
	stateMap, err := h.service.GetStatesByGUIDs(ctx, guids)
	if err != nil {
		return nil, fmt.Errorf("fetch states for edge: %w", err)
	}

	return h.edgeToProtoWithCache(ctx, edge, stateMap)
}

// ListStateOutputs returns cached output keys with sensitive flags for a state.
func (h *StateServiceHandler) ListStateOutputs(
	ctx context.Context,
	req *connect.Request[statev1.ListStateOutputsRequest],
) (*connect.Response[statev1.ListStateOutputsResponse], error) {
	// Resolve state GUID from logic_id or guid
	var guid, logicID string
	if state, ok := req.Msg.State.(*statev1.ListStateOutputsRequest_LogicId); ok {
		logicID = state.LogicId
		// Resolve logic_id to guid
		stateGUID, _, err := h.service.GetStateConfig(ctx, logicID)
		if err != nil {
			return nil, mapServiceError(err)
		}
		guid = stateGUID
	} else if state, ok := req.Msg.State.(*statev1.ListStateOutputsRequest_Guid); ok {
		guid = state.Guid
		// Get logic_id for response
		stateRecord, err := h.service.GetStateByGUID(ctx, guid)
		if err != nil {
			return nil, mapServiceError(err)
		}
		logicID = stateRecord.LogicID
	} else {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required (logic_id or guid)"))
	}

	// Get output keys from service (cache or parse)
	outputs, err := h.service.GetOutputKeys(ctx, guid)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Convert to proto
	protoOutputs := make([]*statev1.OutputKey, len(outputs))
	for i, out := range outputs {
		protoOut := &statev1.OutputKey{
			Key:       out.Key,
			Sensitive: out.Sensitive,
		}
		// Include schema if available
		if out.SchemaJSON != nil && *out.SchemaJSON != "" {
			protoOut.SchemaJson = out.SchemaJSON
		}
		// Include schema source if available
		if out.SchemaSource != nil && *out.SchemaSource != "" {
			protoOut.SchemaSource = out.SchemaSource
		}
		// Include validation status if available
		if out.ValidationStatus != nil && *out.ValidationStatus != "" {
			protoOut.ValidationStatus = out.ValidationStatus
		}
		// Include validation error if available
		if out.ValidationError != nil && *out.ValidationError != "" {
			protoOut.ValidationError = out.ValidationError
		}
		// Include validated_at if available
		if out.ValidatedAt != nil {
			protoOut.ValidatedAt = timestamppb.New(*out.ValidatedAt)
		}
		protoOutputs[i] = protoOut
	}

	resp := &statev1.ListStateOutputsResponse{
		StateGuid:    guid,
		StateLogicId: logicID,
		Outputs:      protoOutputs,
	}

	return connect.NewResponse(resp), nil
}

// GetStateInfo retrieves comprehensive state information including dependencies, dependents, and outputs.
func (h *StateServiceHandler) GetStateInfo(
	ctx context.Context,
	req *connect.Request[statev1.GetStateInfoRequest],
) (*connect.Response[statev1.GetStateInfoResponse], error) {
	// Resolve state reference from logic_id or guid
	var logicID, guid string
	if state, ok := req.Msg.State.(*statev1.GetStateInfoRequest_LogicId); ok {
		logicID = state.LogicId
	} else if state, ok := req.Msg.State.(*statev1.GetStateInfoRequest_Guid); ok {
		guid = state.Guid
	} else {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required (logic_id or guid)"))
	}

	// Get comprehensive state info from service
	info, err := h.service.GetStateInfo(ctx, logicID, guid)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Convert dependencies to proto
	protoDependencies := make([]*statev1.DependencyEdge, 0, len(info.Dependencies))
	for i := range info.Dependencies {
		protoEdge, err := h.edgeToProto(ctx, &info.Dependencies[i], nil)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		protoDependencies = append(protoDependencies, protoEdge)
	}

	// Convert dependents to proto
	protoDependents := make([]*statev1.DependencyEdge, 0, len(info.Dependents))
	for i := range info.Dependents {
		protoEdge, err := h.edgeToProto(ctx, &info.Dependents[i], nil)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		protoDependents = append(protoDependents, protoEdge)
	}

	// Convert outputs to proto
	protoOutputs := make([]*statev1.OutputKey, len(info.Outputs))
	for i, out := range info.Outputs {
		protoOut := &statev1.OutputKey{
			Key:       out.Key,
			Sensitive: out.Sensitive,
		}
		// Include schema if available
		if out.SchemaJSON != nil && *out.SchemaJSON != "" {
			protoOut.SchemaJson = out.SchemaJSON
		}
		// Include schema source if available
		if out.SchemaSource != nil && *out.SchemaSource != "" {
			protoOut.SchemaSource = out.SchemaSource
		}
		// Include validation status if available
		if out.ValidationStatus != nil && *out.ValidationStatus != "" {
			protoOut.ValidationStatus = out.ValidationStatus
		}
		// Include validation error if available
		if out.ValidationError != nil && *out.ValidationError != "" {
			protoOut.ValidationError = out.ValidationError
		}
		// Include validated_at if available
		if out.ValidatedAt != nil {
			protoOut.ValidatedAt = timestamppb.New(*out.ValidatedAt)
		}
		protoOutputs[i] = protoOut
	}

	// Convert labels to proto
	protoLabels := make(map[string]*statev1.LabelValue)
	for k, v := range info.Labels {
		protoLabels[k] = goValueToProtoLabel(v)
	}

	// Build response
	resp := &statev1.GetStateInfoResponse{
		Guid:    info.GUID,
		LogicId: info.LogicID,
		BackendConfig: &statev1.BackendConfig{
			Address:       info.BackendConfig.Address,
			LockAddress:   info.BackendConfig.LockAddress,
			UnlockAddress: info.BackendConfig.UnlockAddress,
		},
		Dependencies: protoDependencies,
		Dependents:   protoDependents,
		Outputs:      protoOutputs,
		SizeBytes:    info.SizeBytes,
		Labels:       protoLabels,
	}

	if !info.CreatedAt.IsZero() {
		resp.CreatedAt = timestamppb.New(info.CreatedAt)
	}
	if !info.UpdatedAt.IsZero() {
		resp.UpdatedAt = timestamppb.New(info.UpdatedAt)
	}

	// Populate computed_status if dependency service is available
	if h.depService != nil {
		status, err := h.depService.GetStateStatus(ctx, info.LogicID, "")
		if err == nil && status != nil {
			resp.ComputedStatus = &status.Status
		}
	}

	return connect.NewResponse(resp), nil
}

// ListAllEdges returns all dependency edges in the system.
// Edges are filtered based on user's role label scopes to ensure users only see edges
// where they have permission to view BOTH the source and destination states.
func (h *StateServiceHandler) ListAllEdges(
	ctx context.Context,
	req *connect.Request[statev1.ListAllEdgesRequest],
) (*connect.Response[statev1.ListAllEdgesResponse], error) {
	if h.depService == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("dependency service not configured"))
	}

	// Get all edges from service
	edges, err := h.depService.ListAllEdges(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Filter edges based on user's role scopes
	// Users only see edges where they can view both source and destination states
	filteredEdges, err := h.filterEdgesByRoleScopes(ctx, edges)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Convert edges to proto
	protoEdges, err := h.edgesToProto(ctx, filteredEdges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&statev1.ListAllEdgesResponse{Edges: protoEdges}), nil
}

// filterEdgesByRoleScopes filters edges based on user's role label scopes.
// An edge is included only if the user has permission to view BOTH the source and destination states.
// This follows the same pattern as filterStatesByRoleScopes in connect_handlers.go.
func (h *StateServiceHandler) filterEdgesByRoleScopes(ctx context.Context, edges []models.Edge) ([]models.Edge, error) {
	// Get principal from context
	principal, ok := auth.GetUserFromContext(ctx)
	if !ok {
		// No principal (no-auth mode) - return all edges
		return edges, nil
	}

	// If IAM service not available, return all edges (backwards compatibility)
	if h.iamService == nil {
		return edges, nil
	}

	// Get role scopes from the user's roles
	roleScopes := make([]string, 0, len(principal.Roles))
	for _, casbinRole := range principal.Roles {
		roleName, err := auth.ExtractRoleID(casbinRole)
		if err != nil {
			// try with casbinRole as is
			roleName = casbinRole
			// TODO: Review Mode 1 vs  Mode 2 role prefix handling?
			log.Printf("warning: could not extract role name from casbin role %s: %v", casbinRole, err)

		}

		role, err := h.iamService.GetRoleByName(ctx, roleName)
		if err != nil {
			continue // Skip roles that don't exist
		}

		roleScopes = append(roleScopes, role.ScopeExpr)
	}

	// If user has no roles, return empty list
	if len(roleScopes) == 0 {
		return []models.Edge{}, nil
	}

	// Build a map of state GUID -> labels for efficient lookup
	// We need to fetch all unique states referenced in the edges
	stateGUIDs := make(map[string]bool)
	for _, edge := range edges {
		stateGUIDs[edge.FromState] = true
		stateGUIDs[edge.ToState] = true
	}

	// Fetch all states
	stateLabels := make(map[string]map[string]any)
	for guid := range stateGUIDs {
		state, err := h.service.GetStateByGUID(ctx, guid)
		if err != nil {
			// State not found - skip (edge will be filtered out)
			continue
		}
		stateLabels[guid] = state.Labels
	}

	// Filter edges: include only if user can see BOTH from and to states
	filtered := make([]models.Edge, 0, len(edges))
	for _, edge := range edges {
		fromLabels, fromExists := stateLabels[edge.FromState]
		toLabels, toExists := stateLabels[edge.ToState]

		// Both states must exist
		if !fromExists || !toExists {
			continue
		}

		// Check if user can see FROM state
		canSeeFrom := false
		for _, scopeExpr := range roleScopes {
			if strings.TrimSpace(scopeExpr) == "" || auth.EvaluateBexpr(scopeExpr, fromLabels) {
				canSeeFrom = true
				break
			}
		}

		// Check if user can see TO state
		canSeeTo := false
		for _, scopeExpr := range roleScopes {
			if strings.TrimSpace(scopeExpr) == "" || auth.EvaluateBexpr(scopeExpr, toLabels) {
				canSeeTo = true
				break
			}
		}

		// Include edge only if user can see both states
		if canSeeFrom && canSeeTo {
			filtered = append(filtered, edge)
		}
	}

	return filtered, nil
}

// SetOutputSchema sets or updates the JSON Schema for a specific state output.
// After setting the schema, triggers validation for that output if it has a value.
func (h *StateServiceHandler) SetOutputSchema(
	ctx context.Context,
	req *connect.Request[statev1.SetOutputSchemaRequest],
) (*connect.Response[statev1.SetOutputSchemaResponse], error) {
	// Resolve state GUID from logic_id or guid
	var guid, logicID string
	if state, ok := req.Msg.State.(*statev1.SetOutputSchemaRequest_StateLogicId); ok {
		logicID = state.StateLogicId
		// Resolve logic_id to guid
		stateGUID, _, err := h.service.GetStateConfig(ctx, logicID)
		if err != nil {
			return nil, mapServiceError(err)
		}
		guid = stateGUID
	} else if state, ok := req.Msg.State.(*statev1.SetOutputSchemaRequest_StateGuid); ok {
		guid = state.StateGuid
		// Get logic_id for response
		stateRecord, err := h.service.GetStateByGUID(ctx, guid)
		if err != nil {
			return nil, mapServiceError(err)
		}
		logicID = stateRecord.LogicID
	} else {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required (state_logic_id or state_guid)"))
	}

	// Set schema and check if output has a value (uses outputs table, not state content)
	// This follows proper layering: handler delegates business logic to service
	outputExists, err := h.service.SetOutputSchemaAndCheckExists(ctx, guid, req.Msg.OutputKey, req.Msg.SchemaJson)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Trigger validation if output exists with a real value (state_serial > 0)
	// Handler only coordinates the job invocation; business logic is in the service
	if h.validationJob != nil && outputExists {
		// Trigger async validation for this output
		go func() {
			// Get the output value from state via service
			val, err := h.service.GetStateOutputValue(context.Background(), guid, req.Msg.OutputKey)
			if err != nil || val == nil {
				return // Can't get value, validation can't run
			}

			// Validate this single output against the schema we just set
			_ = h.validationJob.ValidateOutputs(context.Background(), guid, map[string]any{req.Msg.OutputKey: val})
		}()
	}

	resp := &statev1.SetOutputSchemaResponse{
		Success:      true,
		StateGuid:    guid,
		StateLogicId: logicID,
		OutputKey:    req.Msg.OutputKey,
	}

	return connect.NewResponse(resp), nil
}

// GetOutputSchema retrieves the JSON Schema for a specific state output.
func (h *StateServiceHandler) GetOutputSchema(
	ctx context.Context,
	req *connect.Request[statev1.GetOutputSchemaRequest],
) (*connect.Response[statev1.GetOutputSchemaResponse], error) {
	// Resolve state GUID from logic_id or guid
	var guid, logicID string
	if state, ok := req.Msg.State.(*statev1.GetOutputSchemaRequest_StateLogicId); ok {
		logicID = state.StateLogicId
		// Resolve logic_id to guid
		stateGUID, _, err := h.service.GetStateConfig(ctx, logicID)
		if err != nil {
			return nil, mapServiceError(err)
		}
		guid = stateGUID
	} else if state, ok := req.Msg.State.(*statev1.GetOutputSchemaRequest_StateGuid); ok {
		guid = state.StateGuid
		// Get logic_id for response
		stateRecord, err := h.service.GetStateByGUID(ctx, guid)
		if err != nil {
			return nil, mapServiceError(err)
		}
		logicID = stateRecord.LogicID
	} else {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("state reference required (state_logic_id or state_guid)"))
	}

	// Get the schema via service
	schemaJSON, err := h.service.GetOutputSchema(ctx, guid, req.Msg.OutputKey)
	if err != nil {
		return nil, mapServiceError(err)
	}

	resp := &statev1.GetOutputSchemaResponse{
		StateGuid:    guid,
		StateLogicId: logicID,
		OutputKey:    req.Msg.OutputKey,
		SchemaJson:   schemaJSON,
	}

	return connect.NewResponse(resp), nil
}
