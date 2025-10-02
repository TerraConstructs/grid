package contract

import (
	"testing"

	"connectrpc.com/connect"
)

// TestGetStateConfig validates the GetStateConfig RPC contract
// This test MUST fail until implementation is complete
func TestGetStateConfig(t *testing.T) {
	tests := []struct {
		name    string
		logicID string
		wantErr bool
		errCode connect.Code
	}{
		{
			name:    "success - existing state",
			logicID: "production-us-east",
			wantErr: false,
		},
		{
			name:    "error - non-existent logic_id",
			logicID: "does-not-exist",
			wantErr: true,
			errCode: connect.CodeNotFound,
		},
		{
			name:    "error - empty logic_id",
			logicID: "",
			wantErr: true,
			errCode: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Create Connect client to StateService
			// client := statev1connect.NewStateServiceClient(http.DefaultClient, "http://localhost:8080")

			// TODO: Setup state for success case
			// if !tt.wantErr {
			// 	guid := uuid.Must(uuid.NewV7()).String()
			// 	req := &statev1.CreateStateRequest{Guid: guid, LogicId: tt.logicID}
			// 	_, err := client.CreateState(context.Background(), connect.NewRequest(req))
			// 	require.NoError(t, err)
			// }

			// TODO: Call GetStateConfig RPC
			// req := &statev1.GetStateConfigRequest{LogicId: tt.logicID}
			// resp, err := client.GetStateConfig(context.Background(), connect.NewRequest(req))

			// For now, this test MUST fail
			t.Skip("GetStateConfig not implemented - test will fail")

			// Validation logic (to be uncommented when implementation exists):
			// if tt.wantErr {
			// 	require.Error(t, err)
			// 	assert.Equal(t, tt.errCode, connect.CodeOf(err))
			// } else {
			// 	require.NoError(t, err)
			// 	assert.NotEmpty(t, resp.Msg.Guid)
			// 	assert.NotNil(t, resp.Msg.BackendConfig)
			// 	assert.Contains(t, resp.Msg.BackendConfig.Address, "/tfstate/")
			// }
		})
	}
}
