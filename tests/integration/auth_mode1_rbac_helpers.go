package integration

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ===============================================
// RBAC Helpers
// ===============================================

// assignGroupRoleInGrid assigns a Keycloak group to a Grid role using gridctl CLI
// Idempotent: ignores "already exists" errors for test isolation
func assignGroupRoleInGrid(t *testing.T, bearerToken, groupName, roleName string) {
	t.Helper()

	// Use gridctl role assign-group command with bearer token
	cmd := exec.Command("../../bin/gridctl",
		"--server", serverURL,
		"--token", bearerToken,
		"role", "assign-group", groupName, roleName)

	output, err := cmd.CombinedOutput()

	// Check if this is an "already exists" error (idempotent behavior for test isolation)
	outputStr := string(output)
	if err != nil {
		// If the error is "already_exists", treat it as success
		if strings.Contains(outputStr, "already_exists") || strings.Contains(outputStr, "already assigned") {
			t.Logf("✓ Group '%s' → role '%s' mapping already exists (idempotent)", groupName, roleName)
			return
		}
		// Otherwise, fail the test
		require.NoError(t, err, "gridctl role assign-group failed: %s", outputStr)
	}

	t.Logf("✓ Assigned group '%s' → role '%s'", groupName, roleName)
}
