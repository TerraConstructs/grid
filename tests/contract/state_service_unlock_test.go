package contract

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// TestUnlockState validates the UnlockState RPC contract
// This test verifies programmatic unlock functionality (for automation/recovery)
func TestUnlockState(t *testing.T) {
	tests := []struct {
		name    string
		guid    string
		lockID  string
		wantErr bool
		errCode connect.Code
	}{
		{
			name:    "success - unlock with correct lock ID",
			guid:    uuid.Must(uuid.NewV7()).String(),
			lockID:  uuid.Must(uuid.NewV7()).String(),
			wantErr: false,
		},
		{
			name:    "error - unlock with wrong lock ID",
			guid:    uuid.Must(uuid.NewV7()).String(),
			lockID:  uuid.Must(uuid.NewV7()).String(),
			wantErr: true,
			errCode: connect.CodeInvalidArgument,
		},
		{
			name:    "error - unlock already unlocked state",
			guid:    uuid.Must(uuid.NewV7()).String(),
			lockID:  uuid.Must(uuid.NewV7()).String(),
			wantErr: true,
			errCode: connect.CodeFailedPrecondition,
		},
		{
			name:    "error - non-existent state",
			guid:    uuid.Must(uuid.NewV7()).String(),
			lockID:  uuid.Must(uuid.NewV7()).String(),
			wantErr: true,
			errCode: connect.CodeNotFound,
		},
		{
			name:    "error - invalid guid format",
			guid:    "not-a-uuid",
			lockID:  uuid.Must(uuid.NewV7()).String(),
			wantErr: true,
			errCode: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Create Connect client to StateService
			// client := statev1connect.NewStateServiceClient(http.DefaultClient, "http://localhost:8080")

			// Setup: Create state, lock it (if needed for test scenario)
			// ...

			// TODO: Call UnlockState RPC
			// req := &statev1.UnlockStateRequest{
			// 	Guid:   tt.guid,
			// 	LockId: tt.lockID,
			// }
			// resp, err := client.UnlockState(context.Background(), connect.NewRequest(req))

			// For now, this test MUST fail
			t.Skip("UnlockState not implemented - test will fail")

			// Validation logic (to be uncommented when implementation exists):
			// if tt.wantErr {
			// 	require.Error(t, err)
			// 	assert.Equal(t, tt.errCode, connect.CodeOf(err))
			// } else {
			// 	require.NoError(t, err)
			// 	assert.NotNil(t, resp.Msg.Lock)
			// 	assert.False(t, resp.Msg.Lock.Locked, "State should be unlocked")
			// 	assert.Nil(t, resp.Msg.Lock.Info, "Lock info should be cleared")
			// }
		})
	}
}
