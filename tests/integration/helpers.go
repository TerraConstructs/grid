package integration

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
