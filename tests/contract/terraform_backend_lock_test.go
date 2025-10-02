package contract

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// TestTerraformBackendLOCK validates the LOCK /tfstate/{guid}/lock endpoint contract
// This test MUST fail until implementation is complete
func TestTerraformBackendLOCK(t *testing.T) {
	tests := []struct {
		name           string
		guid           string
		lockInfo       string
		setupState     bool
		setupLock      bool
		wantStatusCode int
	}{
		{
			name:           "success - acquire lock on unlocked state",
			guid:           uuid.Must(uuid.NewV7()).String(),
			lockInfo:       `{"ID":"lock-1","Operation":"apply","Who":"user@host"}`,
			setupState:     true,
			setupLock:      false,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "error - lock conflict (423 Locked)",
			guid:           uuid.Must(uuid.NewV7()).String(),
			lockInfo:       `{"ID":"lock-2","Operation":"apply","Who":"user@host"}`,
			setupState:     true,
			setupLock:      true,
			wantStatusCode: 423, // HTTP 423 Locked
		},
		{
			name:           "error - state not found",
			guid:           uuid.Must(uuid.NewV7()).String(),
			lockInfo:       `{"ID":"lock-3","Operation":"apply","Who":"user@host"}`,
			setupState:     false,
			setupLock:      false,
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Setup test server
			// server := httptest.NewServer(handler)
			// defer server.Close()

			// TODO: Create state if needed
			// if tt.setupState {
			// 	// Create state via Connect RPC
			// }

			// TODO: Setup existing lock if needed
			// if tt.setupLock {
			// 	// Lock state via LOCK endpoint
			// }

			// TODO: Make LOCK request (custom HTTP method)
			// url := server.URL + "/tfstate/" + tt.guid + "/lock"
			// req, _ := http.NewRequest("LOCK", url, bytes.NewReader([]byte(tt.lockInfo)))
			// resp, err := http.DefaultClient.Do(req)

			// For now, this test MUST fail
			t.Skip("Terraform Backend LOCK not implemented - test will fail")

			// Validation logic (to be uncommented when implementation exists):
			// require.NoError(t, err)
			// defer resp.Body.Close()
			// assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
			// if tt.wantStatusCode == 423 {
			// 	// Verify lock info returned in response body
			// 	body, _ := io.ReadAll(resp.Body)
			// 	assert.Contains(t, string(body), "ID")
			// }
		})
	}
}
