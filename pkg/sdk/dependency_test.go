package sdk_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/pkg/api/state/v1"
	"github.com/terraconstructs/grid/pkg/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// Extend mock handler with dependency methods
type mockDependencyHandler struct {
	mockStateServiceHandler
	addDependencyFunc       func(context.Context, *connect.Request[statev1.AddDependencyRequest]) (*connect.Response[statev1.AddDependencyResponse], error)
	removeDependencyFunc    func(context.Context, *connect.Request[statev1.RemoveDependencyRequest]) (*connect.Response[statev1.RemoveDependencyResponse], error)
	listDependenciesFunc    func(context.Context, *connect.Request[statev1.ListDependenciesRequest]) (*connect.Response[statev1.ListDependenciesResponse], error)
	listDependentsFunc      func(context.Context, *connect.Request[statev1.ListDependentsRequest]) (*connect.Response[statev1.ListDependentsResponse], error)
	searchByOutputFunc      func(context.Context, *connect.Request[statev1.SearchByOutputRequest]) (*connect.Response[statev1.SearchByOutputResponse], error)
	getTopologicalOrderFunc func(context.Context, *connect.Request[statev1.GetTopologicalOrderRequest]) (*connect.Response[statev1.GetTopologicalOrderResponse], error)
	getStateStatusFunc      func(context.Context, *connect.Request[statev1.GetStateStatusRequest]) (*connect.Response[statev1.GetStateStatusResponse], error)
	getDependencyGraphFunc  func(context.Context, *connect.Request[statev1.GetDependencyGraphRequest]) (*connect.Response[statev1.GetDependencyGraphResponse], error)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newSDKClient(handler http.Handler, baseURL string) *sdk.Client {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		resp := recorder.Result()
		resp.Request = req
		return resp, nil
	})

	return sdk.NewClient(baseURL, sdk.WithHTTPClient(&http.Client{Transport: transport}))
}

func (m *mockDependencyHandler) AddDependency(ctx context.Context, req *connect.Request[statev1.AddDependencyRequest]) (*connect.Response[statev1.AddDependencyResponse], error) {
	if m.addDependencyFunc != nil {
		return m.addDependencyFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockDependencyHandler) RemoveDependency(ctx context.Context, req *connect.Request[statev1.RemoveDependencyRequest]) (*connect.Response[statev1.RemoveDependencyResponse], error) {
	if m.removeDependencyFunc != nil {
		return m.removeDependencyFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockDependencyHandler) ListDependencies(ctx context.Context, req *connect.Request[statev1.ListDependenciesRequest]) (*connect.Response[statev1.ListDependenciesResponse], error) {
	if m.listDependenciesFunc != nil {
		return m.listDependenciesFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockDependencyHandler) ListDependents(ctx context.Context, req *connect.Request[statev1.ListDependentsRequest]) (*connect.Response[statev1.ListDependentsResponse], error) {
	if m.listDependentsFunc != nil {
		return m.listDependentsFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockDependencyHandler) SearchByOutput(ctx context.Context, req *connect.Request[statev1.SearchByOutputRequest]) (*connect.Response[statev1.SearchByOutputResponse], error) {
	if m.searchByOutputFunc != nil {
		return m.searchByOutputFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockDependencyHandler) GetTopologicalOrder(ctx context.Context, req *connect.Request[statev1.GetTopologicalOrderRequest]) (*connect.Response[statev1.GetTopologicalOrderResponse], error) {
	if m.getTopologicalOrderFunc != nil {
		return m.getTopologicalOrderFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockDependencyHandler) GetStateStatus(ctx context.Context, req *connect.Request[statev1.GetStateStatusRequest]) (*connect.Response[statev1.GetStateStatusResponse], error) {
	if m.getStateStatusFunc != nil {
		return m.getStateStatusFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (m *mockDependencyHandler) GetDependencyGraph(ctx context.Context, req *connect.Request[statev1.GetDependencyGraphRequest]) (*connect.Response[statev1.GetDependencyGraphResponse], error) {
	if m.getDependencyGraphFunc != nil {
		return m.getDependencyGraphFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func TestClient_AddDependency(t *testing.T) {
	handler := &mockDependencyHandler{
		addDependencyFunc: func(_ context.Context, req *connect.Request[statev1.AddDependencyRequest]) (*connect.Response[statev1.AddDependencyResponse], error) {
			toInputName := "producer_vpc_id"
			return connect.NewResponse(&statev1.AddDependencyResponse{
				Edge: &statev1.DependencyEdge{
					Id:          1,
					FromLogicId: "producer",
					FromOutput:  req.Msg.FromOutput,
					ToLogicId:   "consumer",
					ToInputName: &toInputName,
					Status:      "pending",
				},
				AlreadyExists: false,
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")

	result, err := client.AddDependency(context.Background(), sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: "producer"},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: "consumer"},
	})
	if err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	if result.Edge.From.LogicID != "producer" {
		t.Errorf("expected FromLogicId=producer, got %s", result.Edge.From.LogicID)
	}

	if result.Edge.FromOutput != "vpc_id" {
		t.Errorf("expected FromOutput=vpc_id, got %s", result.Edge.FromOutput)
	}
}

func TestClient_RemoveDependency(t *testing.T) {
	handler := &mockDependencyHandler{
		removeDependencyFunc: func(_ context.Context, req *connect.Request[statev1.RemoveDependencyRequest]) (*connect.Response[statev1.RemoveDependencyResponse], error) {
			if req.Msg.EdgeId != 123 {
				return nil, connect.NewError(connect.CodeInvalidArgument, nil)
			}
			return connect.NewResponse(&statev1.RemoveDependencyResponse{}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")

	err := client.RemoveDependency(context.Background(), 123)
	if err != nil {
		t.Fatalf("RemoveDependency failed: %v", err)
	}
}

func TestClient_ListDependencies(t *testing.T) {
	handler := &mockDependencyHandler{
		listDependenciesFunc: func(_ context.Context, req *connect.Request[statev1.ListDependenciesRequest]) (*connect.Response[statev1.ListDependenciesResponse], error) {
			toInputName := "vpc"
			return connect.NewResponse(&statev1.ListDependenciesResponse{
				Edges: []*statev1.DependencyEdge{
					{
						Id:          1,
						FromLogicId: "producer",
						FromOutput:  "vpc_id",
						ToLogicId:   "consumer",
						ToInputName: &toInputName,
						Status:      "clean",
					},
				},
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")

	edges, err := client.ListDependencies(context.Background(), sdk.StateReference{LogicID: "consumer"})
	if err != nil {
		t.Fatalf("ListDependencies failed: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	if edges[0].From.LogicID != "producer" {
		t.Errorf("expected FromLogicId=producer, got %s", edges[0].From.LogicID)
	}
}

func TestClient_ListDependents(t *testing.T) {
	handler := &mockDependencyHandler{
		listDependentsFunc: func(_ context.Context, req *connect.Request[statev1.ListDependentsRequest]) (*connect.Response[statev1.ListDependentsResponse], error) {
			toInputName := "input"
			return connect.NewResponse(&statev1.ListDependentsResponse{
				Edges: []*statev1.DependencyEdge{
					{
						Id:          1,
						FromLogicId: "producer",
						ToLogicId:   "consumer",
						ToInputName: &toInputName,
						Status:      "pending",
					},
				},
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")

	edges, err := client.ListDependents(context.Background(), sdk.StateReference{LogicID: "producer"})
	if err != nil {
		t.Fatalf("ListDependents failed: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
}

func TestClient_SearchByOutput(t *testing.T) {
	handler := &mockDependencyHandler{
		searchByOutputFunc: func(_ context.Context, req *connect.Request[statev1.SearchByOutputRequest]) (*connect.Response[statev1.SearchByOutputResponse], error) {
			toInputName := "vpc"
			return connect.NewResponse(&statev1.SearchByOutputResponse{
				Edges: []*statev1.DependencyEdge{
					{
						Id:          1,
						FromLogicId: "producer1",
						FromOutput:  req.Msg.OutputKey,
						ToInputName: &toInputName,
					},
				},
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")

	edges, err := client.SearchByOutput(context.Background(), "vpc_id")
	if err != nil {
		t.Fatalf("SearchByOutput failed: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	if edges[0].FromOutput != "vpc_id" {
		t.Errorf("expected FromOutput=vpc_id, got %s", edges[0].FromOutput)
	}
}

func TestClient_GetTopologicalOrder(t *testing.T) {
	handler := &mockDependencyHandler{
		getTopologicalOrderFunc: func(_ context.Context, req *connect.Request[statev1.GetTopologicalOrderRequest]) (*connect.Response[statev1.GetTopologicalOrderResponse], error) {
			return connect.NewResponse(&statev1.GetTopologicalOrderResponse{
				Layers: []*statev1.Layer{
					{
						Level: 0,
						States: []*statev1.StateRef{
							{Guid: "guid-a", LogicId: "a"},
						},
					},
				},
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")

	layers, err := client.GetTopologicalOrder(context.Background(), sdk.TopologyInput{
		Root:      sdk.StateReference{LogicID: "a"},
		Direction: sdk.Upstream,
	})
	if err != nil {
		t.Fatalf("GetTopologicalOrder failed: %v", err)
	}

	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}

	if layers[0].Level != 0 {
		t.Errorf("expected level=0, got %d", layers[0].Level)
	}
}

func TestClient_GetStateStatus(t *testing.T) {
	handler := &mockDependencyHandler{
		getStateStatusFunc: func(_ context.Context, req *connect.Request[statev1.GetStateStatusRequest]) (*connect.Response[statev1.GetStateStatusResponse], error) {
			return connect.NewResponse(&statev1.GetStateStatusResponse{
				Guid:    "guid-consumer",
				LogicId: "consumer",
				Status:  "clean",
				Summary: &statev1.StatusSummary{
					IncomingClean:   2,
					IncomingDirty:   0,
					IncomingPending: 0,
					IncomingUnknown: 0,
				},
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")

	status, err := client.GetStateStatus(context.Background(), sdk.StateReference{LogicID: "consumer"})
	if err != nil {
		t.Fatalf("GetStateStatus failed: %v", err)
	}

	if status.Status != "clean" {
		t.Errorf("expected status=clean, got %s", status.Status)
	}

	if status.Summary.IncomingClean != 2 {
		t.Errorf("expected IncomingClean=2, got %d", status.Summary.IncomingClean)
	}
}

func TestClient_GetDependencyGraph(t *testing.T) {
	handler := &mockDependencyHandler{
		getDependencyGraphFunc: func(_ context.Context, req *connect.Request[statev1.GetDependencyGraphRequest]) (*connect.Response[statev1.GetDependencyGraphResponse], error) {
			toInputName := "vpc"
			return connect.NewResponse(&statev1.GetDependencyGraphResponse{
				ConsumerGuid:    "guid-consumer",
				ConsumerLogicId: "consumer",
				Producers: []*statev1.ProducerState{
					{
						Guid:    "guid-producer",
						LogicId: "producer",
						BackendConfig: &statev1.BackendConfig{
							Address:       "http://localhost:8080/tfstate/guid-producer",
							LockAddress:   "http://localhost:8080/tfstate/guid-producer/lock",
							UnlockAddress: "http://localhost:8080/tfstate/guid-producer/unlock",
						},
					},
				},
				Edges: []*statev1.DependencyEdge{
					{
						Id:          1,
						FromLogicId: "producer",
						FromOutput:  "vpc_id",
						ToLogicId:   "consumer",
						ToInputName: &toInputName,
						Status:      "clean",
					},
				},
			}), nil
		},
	}

	mux := http.NewServeMux()
	path, handlerFunc := statev1connect.NewStateServiceHandler(handler)
	mux.Handle(path, handlerFunc)

	client := newSDKClient(mux, "http://example.com")

	graph, err := client.GetDependencyGraph(context.Background(), sdk.StateReference{LogicID: "consumer"})
	if err != nil {
		t.Fatalf("GetDependencyGraph failed: %v", err)
	}

	if graph.Consumer.LogicID != "consumer" {
		t.Errorf("expected ConsumerLogicId=consumer, got %s", graph.Consumer.LogicID)
	}

	if len(graph.Producers) != 1 {
		t.Fatalf("expected 1 producer, got %d", len(graph.Producers))
	}

	if graph.Producers[0].State.LogicID != "producer" {
		t.Errorf("expected producer LogicId=producer, got %s", graph.Producers[0].State.LogicID)
	}

	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
}
