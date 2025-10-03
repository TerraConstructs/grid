package sdk_test

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/pkg/sdk"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockStateServiceHandler implements a test handler for StateService
type mockStateServiceHandler struct {
	statev1connect.UnimplementedStateServiceHandler
	createStateFunc    func(context.Context, *connect.Request[statev1.CreateStateRequest]) (*connect.Response[statev1.CreateStateResponse], error)
	listStatesFunc     func(context.Context, *connect.Request[statev1.ListStatesRequest]) (*connect.Response[statev1.ListStatesResponse], error)
	getStateConfigFunc func(context.Context, *connect.Request[statev1.GetStateConfigRequest]) (*connect.Response[statev1.GetStateConfigResponse], error)
	getStateLockFunc   func(context.Context, *connect.Request[statev1.GetStateLockRequest]) (*connect.Response[statev1.GetStateLockResponse], error)
	unlockStateFunc    func(context.Context, *connect.Request[statev1.UnlockStateRequest]) (*connect.Response[statev1.UnlockStateResponse], error)
}

func (m *mockStateServiceHandler) CreateState(ctx context.Context, req *connect.Request[statev1.CreateStateRequest]) (*connect.Response[statev1.CreateStateResponse], error) {
	if m.createStateFunc != nil {
		return m.createStateFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockStateServiceHandler) ListStates(ctx context.Context, req *connect.Request[statev1.ListStatesRequest]) (*connect.Response[statev1.ListStatesResponse], error) {
	if m.listStatesFunc != nil {
		return m.listStatesFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockStateServiceHandler) GetStateConfig(ctx context.Context, req *connect.Request[statev1.GetStateConfigRequest]) (*connect.Response[statev1.GetStateConfigResponse], error) {
	if m.getStateConfigFunc != nil {
		return m.getStateConfigFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockStateServiceHandler) GetStateLock(ctx context.Context, req *connect.Request[statev1.GetStateLockRequest]) (*connect.Response[statev1.GetStateLockResponse], error) {
	if m.getStateLockFunc != nil {
		return m.getStateLockFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockStateServiceHandler) UnlockState(ctx context.Context, req *connect.Request[statev1.UnlockStateRequest]) (*connect.Response[statev1.UnlockStateResponse], error) {
	if m.unlockStateFunc != nil {
		return m.unlockStateFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func TestClient_CreateState(t *testing.T) {
	tests := []struct {
		name      string
		guid      string
		logicID   string
		mockFunc  func(context.Context, *connect.Request[statev1.CreateStateRequest]) (*connect.Response[statev1.CreateStateResponse], error)
		wantErr   bool
		wantGUID  string
		wantLogic string
	}{
		{
			name:    "success",
			guid:    "018e8c5e-7890-7000-8000-123456789abc",
			logicID: "production-us-east",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.CreateStateRequest]) (*connect.Response[statev1.CreateStateResponse], error) {
				return connect.NewResponse(&statev1.CreateStateResponse{
					Guid:    req.Msg.Guid,
					LogicId: req.Msg.LogicId,
					BackendConfig: &statev1.BackendConfig{
						Address:       "http://localhost:8080/tfstate/" + req.Msg.Guid,
						LockAddress:   "http://localhost:8080/tfstate/" + req.Msg.Guid + "/lock",
						UnlockAddress: "http://localhost:8080/tfstate/" + req.Msg.Guid + "/unlock",
					},
				}), nil
			},
			wantErr:   false,
			wantGUID:  "018e8c5e-7890-7000-8000-123456789abc",
			wantLogic: "production-us-east",
		},
		{
			name:    "duplicate logic_id",
			guid:    "018e8c5e-7890-7000-8000-123456789abc",
			logicID: "production-us-east",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.CreateStateRequest]) (*connect.Response[statev1.CreateStateResponse], error) {
				return nil, connect.NewError(connect.CodeAlreadyExists, nil)
			},
			wantErr: true,
		},
		{
			name:    "invalid guid",
			guid:    "invalid-uuid",
			logicID: "production-us-east",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.CreateStateRequest]) (*connect.Response[statev1.CreateStateResponse], error) {
				return nil, connect.NewError(connect.CodeInvalidArgument, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock handler
			handler := &mockStateServiceHandler{
				createStateFunc: tt.mockFunc,
			}

			// Create test server
			mux := http.NewServeMux()
			path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
			mux.Handle(path, handlerFunc)

			client := newSDKClient(mux, "http://example.com")

			// Execute test
			state, err := client.CreateState(context.Background(), sdk.CreateStateInput{
				GUID:    tt.guid,
				LogicID: tt.logicID,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if state.GUID != tt.wantGUID {
					t.Errorf("CreateState() GUID = %v, want %v", state.GUID, tt.wantGUID)
				}
				if state.LogicID != tt.wantLogic {
					t.Errorf("CreateState() LogicID = %v, want %v", state.LogicID, tt.wantLogic)
				}
				if state.BackendConfig.Address == "" {
					t.Error("CreateState() BackendConfig address is empty")
				}
			}
		})
	}
}

func TestClient_ListStates(t *testing.T) {
	tests := []struct {
		name      string
		mockFunc  func(context.Context, *connect.Request[statev1.ListStatesRequest]) (*connect.Response[statev1.ListStatesResponse], error)
		wantErr   bool
		wantCount int
	}{
		{
			name: "success with states",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.ListStatesRequest]) (*connect.Response[statev1.ListStatesResponse], error) {
				return connect.NewResponse(&statev1.ListStatesResponse{
					States: []*statev1.StateInfo{
						{
							Guid:      "018e8c5e-7890-7000-8000-123456789abc",
							LogicId:   "production-us-east",
							Locked:    false,
							SizeBytes: 1024,
							CreatedAt: timestamppb.Now(),
							UpdatedAt: timestamppb.Now(),
						},
						{
							Guid:      "018e8c5e-7890-7000-8000-123456789def",
							LogicId:   "staging-us-west",
							Locked:    true,
							SizeBytes: 2048,
							CreatedAt: timestamppb.Now(),
							UpdatedAt: timestamppb.Now(),
						},
					},
				}), nil
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name: "success with no states",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.ListStatesRequest]) (*connect.Response[statev1.ListStatesResponse], error) {
				return connect.NewResponse(&statev1.ListStatesResponse{
					States: []*statev1.StateInfo{},
				}), nil
			},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &mockStateServiceHandler{
				listStatesFunc: tt.mockFunc,
			}

			mux := http.NewServeMux()
			path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
			mux.Handle(path, handlerFunc)

			client := newSDKClient(mux, "http://example.com")
			states, err := client.ListStates(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListStates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(states) != tt.wantCount {
				t.Errorf("ListStates() count = %v, want %v", len(states), tt.wantCount)
			}
		})
	}
}

func TestClient_GetState(t *testing.T) {
	tests := []struct {
		name     string
		logicID  string
		mockFunc func(context.Context, *connect.Request[statev1.GetStateConfigRequest]) (*connect.Response[statev1.GetStateConfigResponse], error)
		wantErr  bool
		wantGUID string
	}{
		{
			name:    "success",
			logicID: "production-us-east",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.GetStateConfigRequest]) (*connect.Response[statev1.GetStateConfigResponse], error) {
				return connect.NewResponse(&statev1.GetStateConfigResponse{
					Guid: "018e8c5e-7890-7000-8000-123456789abc",
					BackendConfig: &statev1.BackendConfig{
						Address:       "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc",
						LockAddress:   "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc/lock",
						UnlockAddress: "http://localhost:8080/tfstate/018e8c5e-7890-7000-8000-123456789abc/unlock",
					},
				}), nil
			},
			wantErr:  false,
			wantGUID: "018e8c5e-7890-7000-8000-123456789abc",
		},
		{
			name:    "not found",
			logicID: "nonexistent",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.GetStateConfigRequest]) (*connect.Response[statev1.GetStateConfigResponse], error) {
				return nil, connect.NewError(connect.CodeNotFound, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &mockStateServiceHandler{
				getStateConfigFunc: tt.mockFunc,
			}

			mux := http.NewServeMux()
			path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
			mux.Handle(path, handlerFunc)

			client := newSDKClient(mux, "http://example.com")
			state, err := client.GetState(context.Background(), sdk.StateReference{LogicID: tt.logicID})

			if (err != nil) != tt.wantErr {
				t.Errorf("GetStateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && state.GUID != tt.wantGUID {
				t.Errorf("GetState() GUID = %v, want %v", state.GUID, tt.wantGUID)
			}
		})
	}
}

func TestClient_GetStateLock(t *testing.T) {
	tests := []struct {
		name       string
		guid       string
		mockFunc   func(context.Context, *connect.Request[statev1.GetStateLockRequest]) (*connect.Response[statev1.GetStateLockResponse], error)
		wantErr    bool
		wantLocked bool
	}{
		{
			name: "unlocked state",
			guid: "018e8c5e-7890-7000-8000-123456789abc",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.GetStateLockRequest]) (*connect.Response[statev1.GetStateLockResponse], error) {
				return connect.NewResponse(&statev1.GetStateLockResponse{
					Lock: &statev1.StateLock{
						Locked: false,
					},
				}), nil
			},
			wantErr:    false,
			wantLocked: false,
		},
		{
			name: "locked state",
			guid: "018e8c5e-7890-7000-8000-123456789abc",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.GetStateLockRequest]) (*connect.Response[statev1.GetStateLockResponse], error) {
				return connect.NewResponse(&statev1.GetStateLockResponse{
					Lock: &statev1.StateLock{
						Locked: true,
						Info: &statev1.LockInfo{
							Id:        "lock-123",
							Operation: "apply",
							Who:       "user@example.com",
							Path:      "/path/to/terraform",
						},
					},
				}), nil
			},
			wantErr:    false,
			wantLocked: true,
		},
		{
			name: "state not found",
			guid: "nonexistent",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.GetStateLockRequest]) (*connect.Response[statev1.GetStateLockResponse], error) {
				return nil, connect.NewError(connect.CodeNotFound, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &mockStateServiceHandler{
				getStateLockFunc: tt.mockFunc,
			}

			mux := http.NewServeMux()
			path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
			mux.Handle(path, handlerFunc)

			client := newSDKClient(mux, "http://example.com")
			lock, err := client.GetStateLock(context.Background(), tt.guid)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetStateLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && lock.Locked != tt.wantLocked {
				t.Errorf("GetStateLock() Locked = %v, want %v", lock.Locked, tt.wantLocked)
			}
		})
	}
}

func TestClient_UnlockState(t *testing.T) {
	tests := []struct {
		name     string
		guid     string
		lockID   string
		mockFunc func(context.Context, *connect.Request[statev1.UnlockStateRequest]) (*connect.Response[statev1.UnlockStateResponse], error)
		wantErr  bool
	}{
		{
			name:   "success",
			guid:   "018e8c5e-7890-7000-8000-123456789abc",
			lockID: "lock-123",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.UnlockStateRequest]) (*connect.Response[statev1.UnlockStateResponse], error) {
				return connect.NewResponse(&statev1.UnlockStateResponse{
					Lock: &statev1.StateLock{
						Locked: false,
					},
				}), nil
			},
			wantErr: false,
		},
		{
			name:   "lock id mismatch",
			guid:   "018e8c5e-7890-7000-8000-123456789abc",
			lockID: "wrong-lock-id",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.UnlockStateRequest]) (*connect.Response[statev1.UnlockStateResponse], error) {
				return nil, connect.NewError(connect.CodeInvalidArgument, nil)
			},
			wantErr: true,
		},
		{
			name:   "state not found",
			guid:   "nonexistent",
			lockID: "lock-123",
			mockFunc: func(_ context.Context, req *connect.Request[statev1.UnlockStateRequest]) (*connect.Response[statev1.UnlockStateResponse], error) {
				return nil, connect.NewError(connect.CodeNotFound, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &mockStateServiceHandler{
				unlockStateFunc: tt.mockFunc,
			}

			mux := http.NewServeMux()
			path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
			mux.Handle(path, handlerFunc)

			client := newSDKClient(mux, "http://example.com")
			lock, err := client.UnlockState(context.Background(), tt.guid, tt.lockID)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnlockState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && lock.Locked {
				t.Error("UnlockState() state is still locked after unlock")
			}
		})
	}
}
