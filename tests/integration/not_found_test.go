package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNonExistentStateAccess verifies 404 responses for non-existent states
func TestNonExistentStateAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Generate a random GUID that doesn't exist
	nonExistentGUID := uuid.New().String()

	tests := []struct {
		name           string
		method         string
		url            string
		expectedStatus int
	}{
		{
			name:           "GET non-existent state",
			method:         http.MethodGet,
			url:            fmt.Sprintf("%s/tfstate/%s", serverURL, nonExistentGUID),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "POST to non-existent state",
			method:         http.MethodPost,
			url:            fmt.Sprintf("%s/tfstate/%s", serverURL, nonExistentGUID),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "LOCK non-existent state",
			method:         "LOCK",
			url:            fmt.Sprintf("%s/tfstate/%s/lock", serverURL, nonExistentGUID),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "UNLOCK non-existent state",
			method:         "UNLOCK",
			url:            fmt.Sprintf("%s/tfstate/%s/unlock", serverURL, nonExistentGUID),
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, tt.method, tt.url, nil)
			require.NoError(t, err, "Failed to create request")

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err, "Failed to execute request")
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode,
				"Expected status %d for %s %s", tt.expectedStatus, tt.method, tt.url)

			// Read response body for debugging
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(body))
		})
	}
}
