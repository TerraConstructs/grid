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

// removeGroupRoleInGrid removes a Keycloak group from a Grid role using gridctl CLI
// Idempotent: ignores "not found" errors for cleanup robustness
func removeGroupRoleInGrid(t *testing.T, bearerToken, groupName, roleName string) {
	t.Helper()

	// Use gridctl role remove command with bearer token
	cmd := exec.Command("../../bin/gridctl",
		"--server", serverURL,
		"--token", bearerToken,
		"role", "remove", groupName, roleName)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		// Idempotent: ignore "not found" errors during cleanup
		if strings.Contains(outputStr, "not_found") || strings.Contains(outputStr, "not assigned") {
			t.Logf("✓ Group '%s' → role '%s' mapping already removed", groupName, roleName)
			return
		}
		// Don't fail test cleanup - just log the warning
		t.Logf("Warning: Failed to remove group-role mapping: %s", outputStr)
		return
	}

	t.Logf("✓ Removed group '%s' → role '%s' mapping", groupName, roleName)
}

// assignGroupRoleInGrid assigns a Keycloak group to a Grid role using gridctl CLI
// Idempotent: ignores "already exists" errors for test isolation
// Automatically registers cleanup to remove the mapping after the test completes
func assignGroupRoleInGrid(t *testing.T, bearerToken, groupName, roleName string) {
	t.Helper()

	// Use gridctl role assign command with bearer token
	cmd := exec.Command("../../bin/gridctl",
		"--server", serverURL,
		"--token", bearerToken,
		"role", "assign", groupName, roleName)

	output, err := cmd.CombinedOutput()

	// Check if this is an "already exists" error (idempotent behavior for test isolation)
	outputStr := string(output)
	if err != nil {
		// If the error is "already_exists", treat it as success
		if strings.Contains(outputStr, "already_exists") || strings.Contains(outputStr, "already assigned") {
			t.Logf("✓ Group '%s' → role '%s' mapping already exists (idempotent)", groupName, roleName)
			// Still register cleanup even for existing mappings
			t.Cleanup(func() {
				removeGroupRoleInGrid(t, bearerToken, groupName, roleName)
			})
			return
		}
		// Otherwise, fail the test
		require.NoError(t, err, "gridctl role assign failed: %s", outputStr)
	}

	t.Logf("✓ Assigned group '%s' → role '%s'", groupName, roleName)

	// Register cleanup to remove mapping after test completes
	t.Cleanup(func() {
		removeGroupRoleInGrid(t, bearerToken, groupName, roleName)
	})
}
