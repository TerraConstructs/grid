package integration

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/terraconstructs/grid/pkg/sdk"
)

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
