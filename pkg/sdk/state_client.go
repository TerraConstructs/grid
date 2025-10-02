package sdk

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
)

// Client provides a high-level interface to the Grid state management API.
// It wraps the generated Connect RPC client with ergonomic methods.
type Client struct {
	rpc statev1connect.StateServiceClient
}

// NewClient creates a new Grid SDK client that communicates with the API server at baseURL.
// The httpClient parameter allows customization of HTTP behavior (timeouts, retries, etc.).
func NewClient(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	rpcClient := statev1connect.NewStateServiceClient(httpClient, baseURL)

	return &Client{
		rpc: rpcClient,
	}
}

// CreateState creates a new Terraform state with the given GUID (client-generated UUIDv7)
// and logic_id (user-provided human-readable identifier).
// Returns the created state information and Terraform backend configuration.
func (c *Client) CreateState(ctx context.Context, guid, logicID string) (*statev1.CreateStateResponse, error) {
	req := connect.NewRequest(&statev1.CreateStateRequest{
		Guid:    guid,
		LogicId: logicID,
	})

	resp, err := c.rpc.CreateState(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Msg, nil
}

// ListStates returns summary information for all states managed by the server.
func (c *Client) ListStates(ctx context.Context) (*statev1.ListStatesResponse, error) {
	req := connect.NewRequest(&statev1.ListStatesRequest{})

	resp, err := c.rpc.ListStates(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Msg, nil
}

// GetStateConfig retrieves the Terraform backend configuration for an existing state
// identified by its logic_id.
func (c *Client) GetStateConfig(ctx context.Context, logicID string) (*statev1.GetStateConfigResponse, error) {
	req := connect.NewRequest(&statev1.GetStateConfigRequest{
		LogicId: logicID,
	})

	resp, err := c.rpc.GetStateConfig(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Msg, nil
}

// GetStateLock inspects the current lock status and metadata for a state by its GUID.
// Returns lock information if the state is locked, or a StateLock with Locked=false if unlocked.
func (c *Client) GetStateLock(ctx context.Context, guid string) (*statev1.GetStateLockResponse, error) {
	req := connect.NewRequest(&statev1.GetStateLockRequest{
		Guid: guid,
	})

	resp, err := c.rpc.GetStateLock(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Msg, nil
}

// UnlockState releases a lock on a state identified by GUID.
// The lockID must match the ID of the current lock, or the operation will fail.
func (c *Client) UnlockState(ctx context.Context, guid, lockID string) (*statev1.UnlockStateResponse, error) {
	req := connect.NewRequest(&statev1.UnlockStateRequest{
		Guid:   guid,
		LockId: lockID,
	})

	resp, err := c.rpc.UnlockState(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Msg, nil
}
