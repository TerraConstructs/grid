package server

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/state"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StateServiceHandler wires the internal state service to Connect RPC contracts.
type StateServiceHandler struct {
	statev1connect.UnimplementedStateServiceHandler
	service *statepkg.Service
}

// NewStateServiceHandler constructs a handler backed by the provided service.
func NewStateServiceHandler(service *statepkg.Service) *StateServiceHandler {
	return &StateServiceHandler{service: service}
}

// CreateState creates a new state with client-generated GUID and logic_id.
func (h *StateServiceHandler) CreateState(
	ctx context.Context,
	req *connect.Request[statev1.CreateStateRequest],
) (*connect.Response[statev1.CreateStateResponse], error) {
	summary, config, err := h.service.CreateState(ctx, req.Msg.Guid, req.Msg.LogicId)
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
func (h *StateServiceHandler) ListStates(
	ctx context.Context,
	req *connect.Request[statev1.ListStatesRequest],
) (*connect.Response[statev1.ListStatesResponse], error) {
	summaries, err := h.service.ListStates(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	infos := make([]*statev1.StateInfo, 0, len(summaries))
	for _, summary := range summaries {
		infos = append(infos, summaryToProto(summary))
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
	case strings.Contains(msg, "already exists"):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case strings.Contains(msg, "invalid"), strings.Contains(msg, "required"), strings.Contains(msg, "guid"):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case strings.Contains(msg, "not found"):
		return connect.NewError(connect.CodeNotFound, err)
	case strings.Contains(msg, "locked"):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
