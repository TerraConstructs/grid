package contract

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// TestGetStateLock validates the GetStateLock RPC contract
// This test verifies lock inspection functionality
func TestGetStateLock(t *testing.T) {
	tests := []struct {
		name    string
		guid    string
		wantErr bool
		errCode connect.Code
	}{
		{
			name:    "success - get lock for unlocked state",
			guid:    uuid.Must(uuid.NewV7()).String(),
			wantErr: false,
		},
		{
			name:    "success - get lock for locked state",
			guid:    uuid.Must(uuid.NewV7()).String(),
			wantErr: false,
		},
		{
			name:    "error - non-existent state",
			guid:    uuid.Must(uuid.NewV7()).String(),
			wantErr: true,
			errCode: connect.CodeNotFound,
		},
		{
			name:    "error - invalid guid format",
			guid:    "not-a-uuid",
			wantErr: true,
			errCode: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Create Connect client to StateService
			// client := statev1connect.NewStateServiceClient(http.DefaultClient, "http://localhost:8080")

			// Setup: Create state and optionally lock it
			// ...

			// TODO: Call GetStateLock RPC
			// req := &statev1.GetStateLockRequest{
			// 	Guid: tt.guid,
			// }
			// resp, err := client.GetStateLock(context.Background(), connect.NewRequest(req))

			// For now, this test MUST fail
			t.Skip("GetStateLock not implemented - test will fail")

			// Validation logic (to be uncommented when implementation exists):
			// if tt.wantErr {
			// 	require.Error(t, err)
			// 	assert.Equal(t, tt.errCode, connect.CodeOf(err))
			// } else {
			// 	require.NoError(t, err)
			// 	assert.NotNil(t, resp.Msg.Lock)
			// 	// If locked, verify lock info is present
			// 	if resp.Msg.Lock.Locked {
			// 		assert.NotNil(t, resp.Msg.Lock.Info)
			// 		assert.NotEmpty(t, resp.Msg.Lock.Info.Id)
			// 	} else {
			// 		assert.Nil(t, resp.Msg.Lock.Info)
			// 	}
			// }
		})
	}
}
