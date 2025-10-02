package integration

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStateSizeWarning verifies that large state files trigger warnings
func TestStateSizeWarning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	logicID := fmt.Sprintf("test-size-%s", uuid.New().String()[:8])

	// Step 1: Create state via gridctl
	t.Logf("Creating state with logic_id: %s", logicID)
	createCmd := exec.CommandContext(ctx, getGridctlPath(t), "state", "create", logicID, "--server", serverURL)
	createOut, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create state: %s", string(createOut))

	// Extract GUID from output
	output := string(createOut)
	lines := strings.Split(output, "\n")
	var guid string
	for _, line := range lines {
		if strings.Contains(line, "Created state:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				guid = parts[2]
				break
			}
		}
	}
	require.NotEmpty(t, guid, "Failed to extract GUID from create output")

	// Step 2: Create a large state content (>10MB)
	// Generate a realistic Terraform state structure
	largeStateContent := generateLargeState(11 * 1024 * 1024) // 11MB

	// Step 3: POST the large state
	t.Logf("Posting large state content (%d bytes)", len(largeStateContent))
	url := fmt.Sprintf("%s/tfstate/%s", serverURL, guid)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(largeStateContent))
	require.NoError(t, err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to execute request")
	defer resp.Body.Close()

	// Step 4: Verify response includes size warning header
	assert.Equal(t, http.StatusOK, resp.StatusCode, "POST should succeed despite size")

	warningHeader := resp.Header.Get("X-Grid-State-Size-Warning")
	assert.NotEmpty(t, warningHeader, "Should include size warning header")
	t.Logf("Size warning header: %s", warningHeader)

	// Step 5: Verify state was still stored
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err, "Failed to create GET request")

	getResp, err := client.Do(getReq)
	require.NoError(t, err, "Failed to execute GET request")
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusOK, getResp.StatusCode, "Should be able to retrieve large state")
}

// generateLargeState creates a realistic Terraform state JSON of specified size
func generateLargeState(targetSize int) []byte {
	// Start with a minimal valid Terraform state
	header := `{
  "version": 4,
  "terraform_version": "1.5.0",
  "serial": 1,
  "lineage": "00000000-0000-0000-0000-000000000000",
  "outputs": {},
  "resources": [`

	footer := `
  ],
  "check_results": null
}`

	// Calculate base size
	baseSize := len(header) + len(footer)

	var buffer bytes.Buffer
	buffer.WriteString(header)

	// Add resources until we reach target size
	resourceTemplate := `
    {
      "mode": "managed",
      "type": "null_resource",
      "name": "padding_%d",
      "provider": "provider[\"registry.terraform.io/hashicorp/null\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "id": "%s",
            "triggers": {
              "data": "%s"
            }
          }
        }
      ]
    }`

	resourceCount := 0
	currentSize := baseSize

	for currentSize < targetSize {
		if resourceCount > 0 {
			buffer.WriteString(",")
		}

		// Generate padding data
		paddingData := strings.Repeat("x", 1000)
		resourceJSON := fmt.Sprintf(resourceTemplate, resourceCount, uuid.New().String(), paddingData)

		buffer.WriteString(resourceJSON)
		currentSize += len(resourceJSON)
		resourceCount++

		// Safety check to avoid infinite loop
		if resourceCount > 100000 {
			break
		}
	}

	buffer.WriteString(footer)

	return buffer.Bytes()
}
