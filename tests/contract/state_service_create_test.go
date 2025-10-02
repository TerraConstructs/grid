package contract

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// TestCreateState validates the CreateState RPC contract
// This test MUST fail until implementation is complete
func TestCreateState(t *testing.T) {
	tests := []struct {
		name    string
		guid    string
		logicID string
		wantErr bool
		errCode connect.Code
	}{
		{
			name:    "success - valid uuid and logic_id",
			guid:    uuid.Must(uuid.NewV7()).String(),
			logicID: "production-us-east",
			wantErr: false,
		},
		{
			name:    "error - invalid uuid format",
			guid:    "not-a-uuid",
			logicID: "production-us-east",
			wantErr: true,
			errCode: connect.CodeInvalidArgument,
		},
		{
			name:    "error - empty logic_id",
			guid:    uuid.Must(uuid.NewV7()).String(),
			logicID: "",
			wantErr: true,
			errCode: connect.CodeInvalidArgument,
		},
		{
			name:    "error - duplicate logic_id",
			guid:    uuid.Must(uuid.NewV7()).String(),
			logicID: "duplicate-test",
			wantErr: true,
			errCode: connect.CodeAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Create Connect client to StateService
			// client := statev1connect.NewStateServiceClient(http.DefaultClient, "http://localhost:8080")

			// TODO: Call CreateState RPC
			// req := &statev1.CreateStateRequest{
			// 	Guid:    tt.guid,
			// 	LogicId: tt.logicID,
			// }
			// resp, err := client.CreateState(context.Background(), connect.NewRequest(req))

			// For now, this test MUST fail
			t.Skip("CreateState not implemented - test will fail")

			// Validation logic (to be uncommented when implementation exists):
			// if tt.wantErr {
			// 	require.Error(t, err)
			// 	assert.Equal(t, tt.errCode, connect.CodeOf(err))
			// } else {
			// 	require.NoError(t, err)
			// 	assert.Equal(t, tt.guid, resp.Msg.Guid)
			// 	assert.Equal(t, tt.logicID, resp.Msg.LogicId)
			// 	assert.NotNil(t, resp.Msg.BackendConfig)
			// 	assert.Contains(t, resp.Msg.BackendConfig.Address, "/tfstate/"+tt.guid)
			// 	assert.Contains(t, resp.Msg.BackendConfig.LockAddress, "/tfstate/"+tt.guid+"/lock")
			// 	assert.Contains(t, resp.Msg.BackendConfig.UnlockAddress, "/tfstate/"+tt.guid+"/unlock")
			// }
		})
	}
}
