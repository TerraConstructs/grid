package integration

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDuplicateLogicID verifies that duplicate logic_id creation fails appropriately
func TestDuplicateLogicID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	logicID := fmt.Sprintf("test-dup-%s", uuid.New().String()[:8])

	// Step 1: Create first state with the logic_id
	t.Logf("Creating first state with logic_id: %s", logicID)
	createCmd1 := exec.CommandContext(ctx, getGridctlPath(t), "state", "create", logicID, "--server", serverURL)
	createOut1, err := createCmd1.CombinedOutput()
	require.NoError(t, err, "First create should succeed: %s", string(createOut1))
	t.Logf("First create output: %s", string(createOut1))

	// Step 2: Try to create second state with same logic_id (should fail)
	t.Logf("Attempting to create duplicate state with logic_id: %s", logicID)
	createCmd2 := exec.CommandContext(ctx, getGridctlPath(t), "state", "create", logicID, "--server", serverURL)
	createOut2, err := createCmd2.CombinedOutput()

	// Verify that the second create fails
	assert.Error(t, err, "Second create with duplicate logic_id should fail")

	output := string(createOut2)
	t.Logf("Second create output: %s", output)

	// Check for error message about duplicate
	assert.True(t,
		strings.Contains(strings.ToLower(output), "already exists") ||
			strings.Contains(strings.ToLower(output), "duplicate") ||
			strings.Contains(strings.ToLower(output), "conflict") ||
			strings.Contains(output, "409"),
		"Error message should indicate duplicate/conflict: %s", output)

	// Step 3: Verify that only one state exists with this logic_id
	listCmd := exec.CommandContext(ctx, getGridctlPath(t), "state", "list", "--server", serverURL)
	listOut, err := listCmd.CombinedOutput()
	require.NoError(t, err, "List should succeed: %s", string(listOut))

	// Count occurrences of the logic_id in the output
	occurrences := strings.Count(string(listOut), logicID)
	assert.Equal(t, 1, occurrences, "Should only have one state with logic_id %s", logicID)

	// Step 4: Verify we can still init with the existing logic_id
	tmpDir := t.TempDir()
	initCmd := exec.CommandContext(ctx, getGridctlPath(t), "state", "init", logicID, "--server", serverURL)
	initCmd.Dir = tmpDir
	initOut, err := initCmd.CombinedOutput()
	require.NoError(t, err, "Init with existing logic_id should succeed: %s", string(initOut))
}
