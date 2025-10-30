package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/terraconstructs/grid/pkg/sdk"
)

// serverURL is the base URL for the Grid API server in tests
const serverURL = "http://localhost:8080"

// healthResponse represents the response from /health endpoint
type healthResponse struct {
	Status      string `json:"status"`
	OIDCEnabled bool   `json:"oidc_enabled"`
}

// getGridctlPath returns the absolute path to the gridctl binary
func getGridctlPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../bin/gridctl")
	require.NoError(t, err, "Failed to get absolute path to gridctl")
	return path
}

// getGridAPIPath returns the absolute path to the gridapi binary
func getGridAPIPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../bin/gridapi")
	require.NoError(t, err, "Failed to get absolute path to gridapi")
	return path
}

// runGridctl executes the gridctl CLI with the given arguments and optional working directory.
func runGridctl(t *testing.T, ctx context.Context, workDir string, args ...string) (string, error) {
	gridctlPath := getGridctlPath(t)
	fullArgs := append(args, "--server", serverURL)
	cmd := exec.CommandContext(ctx, gridctlPath, fullArgs...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func mustRunGridctl(t *testing.T, ctx context.Context, workDir string, args ...string) string {
	output, err := runGridctl(t, ctx, workDir, args...)
	if err != nil {
		t.Fatalf("gridctl %v failed: %v\n%s", args, err, output)
	}
	return output
}

func newSDKClient() *sdk.Client {
	return sdk.NewClient(serverURL)
}

// getHealthStatus fetches and parses the /health endpoint
func getHealthStatus(t *testing.T) *healthResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	healthURL := fmt.Sprintf("%s/health", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	require.NoError(t, err, "Failed to create health request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call health endpoint")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read health response")

	var health healthResponse
	err = json.Unmarshal(body, &health)
	require.NoError(t, err, "Failed to parse health response: %s", string(body))

	return &health
}

// verifyAuthEnabled checks that OIDC authentication is enabled on the server
// This prevents tests from passing when auth is accidentally disabled
func verifyAuthEnabled(t *testing.T, mode string) {
	t.Helper()
	health := getHealthStatus(t)

	if !health.OIDCEnabled {
		t.Fatalf("CRITICAL: Authentication is NOT enabled on the server!\n"+
			"Health endpoint returned: %+v\n"+
			"Expected: oidc_enabled=true for %s tests\n"+
			"\n"+
			"This means the server is running WITHOUT authentication, which would cause\n"+
			"all auth tests to silently pass without actually testing auth.\n"+
			"\n"+
			"Fix: Ensure environment variables are set correctly:\n"+
			"  Mode 1: EXTERNAL_IDP_ISSUER, EXTERNAL_IDP_CLIENT_ID, EXTERNAL_IDP_CLIENT_SECRET\n"+
			"  Mode 2: OIDC_ISSUER, OIDC_SIGNING_KEY_PATH\n"+
			"\n"+
			"Check that main_test.go passes env vars to the server via serverCmd.Env",
			health, mode)
	}

	t.Logf("✓ Authentication verified: oidc_enabled=%t (mode=%s)", health.OIDCEnabled, mode)
}

// getTerraformAuthEnv returns environment variables for Terraform HTTP Backend authentication
// The token is passed as both TF_HTTP_USERNAME and TF_HTTP_PASSWORD as required by the backend
func getTerraformAuthEnv(token string) []string {
	return []string{
		fmt.Sprintf("TF_HTTP_USERNAME=%s", "gridapi"), // ignored by Grid API
		fmt.Sprintf("TF_HTTP_PASSWORD=%s", token),
	}
}
