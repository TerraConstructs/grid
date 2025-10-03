package sdk

import (
	"context"
	"fmt"
	"net/http"

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

	req := connect.NewRequest(&statev1.CreateStateRequest{
		Guid:    guid,
		LogicId: input.LogicID,
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
	req := connect.NewRequest(&statev1.ListStatesRequest{})

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
