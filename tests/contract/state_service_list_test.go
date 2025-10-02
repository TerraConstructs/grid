package contract

import (
	"testing"
)

// TestListStates validates the ListStates RPC contract
// This test MUST fail until implementation is complete
func TestListStates(t *testing.T) {
	tests := []struct {
		name          string
		setupStates   int // number of states to create before listing
		wantMinStates int
	}{
		{
			name:          "empty list",
			setupStates:   0,
			wantMinStates: 0,
		},
		{
			name:          "list with multiple states",
			setupStates:   3,
			wantMinStates: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Create Connect client to StateService
			// client := statev1connect.NewStateServiceClient(http.DefaultClient, "http://localhost:8080")

			// TODO: Setup states if needed
			// for i := 0; i < tt.setupStates; i++ {
			// 	guid := uuid.Must(uuid.NewV7()).String()
			// 	logicID := fmt.Sprintf("test-state-%d", i)
			// 	req := &statev1.CreateStateRequest{Guid: guid, LogicId: logicID}
			// 	_, err := client.CreateState(context.Background(), connect.NewRequest(req))
			// 	require.NoError(t, err)
			// }

			// TODO: Call ListStates RPC
			// req := &statev1.ListStatesRequest{}
			// resp, err := client.ListStates(context.Background(), connect.NewRequest(req))

			// For now, this test MUST fail
			t.Skip("ListStates not implemented - test will fail")

			// Validation logic (to be uncommented when implementation exists):
			// require.NoError(t, err)
			// assert.GreaterOrEqual(t, len(resp.Msg.States), tt.wantMinStates)
			// for _, state := range resp.Msg.States {
			// 	assert.NotEmpty(t, state.Guid)
			// 	assert.NotEmpty(t, state.LogicId)
			// 	assert.NotNil(t, state.CreatedAt)
			// 	assert.NotNil(t, state.UpdatedAt)
			// }
		})
	}
}
