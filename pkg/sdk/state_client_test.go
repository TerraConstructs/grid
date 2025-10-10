package sdk_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/pkg/sdk"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockStateServiceHandler implements a test handler for StateService
type mockStateServiceHandler struct {
	statev1connect.UnimplementedStateServiceHandler
	createStateFunc       func(context.Context, *connect.Request[statev1.CreateStateRequest]) (*connect.Response[statev1.CreateStateResponse], error)
	listStatesFunc        func(context.Context, *connect.Request[statev1.ListStatesRequest]) (*connect.Response[statev1.ListStatesResponse], error)
	getStateConfigFunc    func(context.Context, *connect.Request[statev1.GetStateConfigRequest]) (*connect.Response[statev1.GetStateConfigResponse], error)
	getStateInfoFunc      func(context.Context, *connect.Request[statev1.GetStateInfoRequest]) (*connect.Response[statev1.GetStateInfoResponse], error)
	getStateLockFunc      func(context.Context, *connect.Request[statev1.GetStateLockRequest]) (*connect.Response[statev1.GetStateLockResponse], error)
	unlockStateFunc       func(context.Context, *connect.Request[statev1.UnlockStateRequest]) (*connect.Response[statev1.UnlockStateResponse], error)
	updateStateLabelsFunc func(context.Context, *connect.Request[statev1.UpdateStateLabelsRequest]) (*connect.Response[statev1.UpdateStateLabelsResponse], error)
	getLabelPolicyFunc    func(context.Context, *connect.Request[statev1.GetLabelPolicyRequest]) (*connect.Response[statev1.GetLabelPolicyResponse], error)
	setLabelPolicyFunc    func(context.Context, *connect.Request[statev1.SetLabelPolicyRequest]) (*connect.Response[statev1.SetLabelPolicyResponse], error)
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

func (m *mockStateServiceHandler) GetStateInfo(ctx context.Context, req *connect.Request[statev1.GetStateInfoRequest]) (*connect.Response[statev1.GetStateInfoResponse], error) {
	if m.getStateInfoFunc != nil {
		return m.getStateInfoFunc(ctx, req)
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

func (m *mockStateServiceHandler) UpdateStateLabels(ctx context.Context, req *connect.Request[statev1.UpdateStateLabelsRequest]) (*connect.Response[statev1.UpdateStateLabelsResponse], error) {
	if m.updateStateLabelsFunc != nil {
		return m.updateStateLabelsFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockStateServiceHandler) GetLabelPolicy(ctx context.Context, req *connect.Request[statev1.GetLabelPolicyRequest]) (*connect.Response[statev1.GetLabelPolicyResponse], error) {
	if m.getLabelPolicyFunc != nil {
		return m.getLabelPolicyFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockStateServiceHandler) SetLabelPolicy(ctx context.Context, req *connect.Request[statev1.SetLabelPolicyRequest]) (*connect.Response[statev1.SetLabelPolicyResponse], error) {
	if m.setLabelPolicyFunc != nil {
		return m.setLabelPolicyFunc(ctx, req)
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
							Labels: map[string]*statev1.LabelValue{
								"env": {Value: &statev1.LabelValue_StringValue{StringValue: "prod"}},
							},
						},
						{
							Guid:      "018e8c5e-7890-7000-8000-123456789def",
							LogicId:   "staging-us-west",
							Locked:    true,
							SizeBytes: 2048,
							CreatedAt: timestamppb.Now(),
							UpdatedAt: timestamppb.Now(),
							Labels:    map[string]*statev1.LabelValue{},
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
			if !tt.wantErr && tt.wantCount > 0 {
				if states[0].Labels["env"].(string) != "prod" {
					t.Errorf("expected env label to be prod, got %v", states[0].Labels["env"])
				}
			}
		})
	}
}

func TestClient_ListStatesWithOptions(t *testing.T) {
	include := false
	var capturedFilter string
	var capturedInclude *bool

	handler := &mockStateServiceHandler{
		listStatesFunc: func(_ context.Context, req *connect.Request[statev1.ListStatesRequest]) (*connect.Response[statev1.ListStatesResponse], error) {
			if req.Msg.Filter != nil {
				capturedFilter = *req.Msg.Filter
			}
			capturedInclude = req.Msg.IncludeLabels
			return connect.NewResponse(&statev1.ListStatesResponse{States: []*statev1.StateInfo{}}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")
	_, err := client.ListStatesWithOptions(context.Background(), sdk.ListStatesOptions{
		Filter:        "env == \"prod\"",
		IncludeLabels: &include,
	})
	if err != nil {
		t.Fatalf("ListStatesWithOptions returned error: %v", err)
	}
	if capturedFilter != "env == \"prod\"" {
		t.Fatalf("expected filter to be captured, got %q", capturedFilter)
	}
	if capturedInclude == nil || *capturedInclude != include {
		t.Fatalf("expected include_labels pointer to be %v", include)
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

func TestClient_GetStateInfo(t *testing.T) {
	created := timestamppb.New(time.Unix(1700000000, 0))
	updated := timestamppb.New(time.Unix(1700003600, 0))

	handler := &mockStateServiceHandler{
		getStateInfoFunc: func(_ context.Context, req *connect.Request[statev1.GetStateInfoRequest]) (*connect.Response[statev1.GetStateInfoResponse], error) {
			if logicID := req.Msg.GetLogicId(); logicID != "production-us-east" {
				return nil, connect.NewError(connect.CodeNotFound, nil)
			}
			return connect.NewResponse(&statev1.GetStateInfoResponse{
				Guid:    "018e8c5e-7890-7000-8000-123456789abc",
				LogicId: "production-us-east",
				BackendConfig: &statev1.BackendConfig{
					Address:       "http://example.com/backend",
					LockAddress:   "http://example.com/backend/lock",
					UnlockAddress: "http://example.com/backend/unlock",
				},
				Outputs: []*statev1.OutputKey{
					{Key: "database_password", Sensitive: true},
				},
				CreatedAt: created,
				UpdatedAt: updated,
				SizeBytes: 4096,
				Labels: map[string]*statev1.LabelValue{
					"env":    {Value: &statev1.LabelValue_StringValue{StringValue: "prod"}},
					"active": {Value: &statev1.LabelValue_BoolValue{BoolValue: true}},
				},
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")
	info, err := client.GetStateInfo(context.Background(), sdk.StateReference{LogicID: "production-us-east"})
	if err != nil {
		t.Fatalf("GetStateInfo() unexpected error: %v", err)
	}

	if info.SizeBytes != 4096 {
		t.Errorf("GetStateInfo() SizeBytes = %d, want 4096", info.SizeBytes)
	}

	if len(info.Outputs) != 1 || info.Outputs[0].Key != "database_password" || !info.Outputs[0].Sensitive {
		t.Errorf("GetStateInfo() Outputs not converted correctly: %+v", info.Outputs)
	}

	env, ok := info.Labels["env"].(string)
	if !ok || env != "prod" {
		t.Errorf("GetStateInfo() Labels[env] = %v, want prod", info.Labels["env"])
	}

	active, ok := info.Labels["active"].(bool)
	if !ok || !active {
		t.Errorf("GetStateInfo() Labels[active] = %v, want true", info.Labels["active"])
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

func TestClient_UpdateStateLabels(t *testing.T) {
	var receivedAdds map[string]*statev1.LabelValue
	var receivedRemovals []string

	handler := &mockStateServiceHandler{
		updateStateLabelsFunc: func(_ context.Context, req *connect.Request[statev1.UpdateStateLabelsRequest]) (*connect.Response[statev1.UpdateStateLabelsResponse], error) {
			receivedAdds = req.Msg.GetAdds()
			receivedRemovals = req.Msg.GetRemovals()
			return connect.NewResponse(&statev1.UpdateStateLabelsResponse{
				StateId: "state-123",
				Labels: map[string]*statev1.LabelValue{
					"env": {Value: &statev1.LabelValue_StringValue{StringValue: "prod"}},
				},
				PolicyVersion: 2,
				UpdatedAt:     timestamppb.Now(),
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")
	result, err := client.UpdateStateLabels(context.Background(), sdk.UpdateStateLabelsInput{
		StateID:  "state-123",
		Adds:     sdk.LabelMap{"env": "prod"},
		Removals: []string{"old"},
	})
	if err != nil {
		t.Fatalf("UpdateStateLabels returned error: %v", err)
	}
	if receivedAdds["env"].GetStringValue() != "prod" {
		t.Fatalf("expected adds env=prod, got %v", receivedAdds["env"])
	}
	if len(receivedRemovals) != 1 || receivedRemovals[0] != "old" {
		t.Fatalf("expected removals ['old'], got %v", receivedRemovals)
	}
	if result.Labels["env"].(string) != "prod" {
		t.Fatalf("expected result label env=prod, got %v", result.Labels["env"])
	}
	if result.PolicyVersion != 2 {
		t.Fatalf("expected policy version 2, got %d", result.PolicyVersion)
	}
}

func TestClient_GetLabelPolicy(t *testing.T) {
	handler := &mockStateServiceHandler{
		getLabelPolicyFunc: func(_ context.Context, req *connect.Request[statev1.GetLabelPolicyRequest]) (*connect.Response[statev1.GetLabelPolicyResponse], error) {
			return connect.NewResponse(&statev1.GetLabelPolicyResponse{
				Version:    3,
				PolicyJson: `{"allowed_keys":{"env":{}}}`,
				CreatedAt:  timestamppb.Now(),
				UpdatedAt:  timestamppb.Now(),
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")
	policy, err := client.GetLabelPolicy(context.Background())
	if err != nil {
		t.Fatalf("GetLabelPolicy returned error: %v", err)
	}
	if policy.Version != 3 {
		t.Fatalf("expected version 3, got %d", policy.Version)
	}
	if policy.PolicyJSON == "" {
		t.Fatal("expected policy JSON to be populated")
	}
}

func TestClient_SetLabelPolicy(t *testing.T) {
	handler := &mockStateServiceHandler{
		setLabelPolicyFunc: func(_ context.Context, req *connect.Request[statev1.SetLabelPolicyRequest]) (*connect.Response[statev1.SetLabelPolicyResponse], error) {
			if req.Msg.PolicyJson == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, nil)
			}
			return connect.NewResponse(&statev1.SetLabelPolicyResponse{
				Version:   4,
				UpdatedAt: timestamppb.Now(),
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")
	policy, err := client.SetLabelPolicy(context.Background(), []byte(`{"max_keys":32}`))
	if err != nil {
		t.Fatalf("SetLabelPolicy returned error: %v", err)
	}
	if policy.Version != 4 {
		t.Fatalf("expected version 4, got %d", policy.Version)
	}
	if policy.PolicyJSON == "" {
		t.Fatal("expected policy JSON to echo request")
	}
}
