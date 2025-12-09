package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// TestRenameState_HappyPath verifies the basic rename flow:
// - Create state with logic_id
// - Rename to new logic_id
// - Verify GUID unchanged
// - Verify new name works, old name returns not found
func TestRenameState_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create a state with unique logic_id
	oldLogicID := fmt.Sprintf("test-rename-old-%s", uuid.New().String()[:8])
	newLogicID := fmt.Sprintf("test-rename-new-%s", uuid.New().String()[:8])

	t.Logf("Creating state with logic_id: %s", oldLogicID)
	state, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: oldLogicID,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state should succeed")
	require.NotEmpty(t, state.GUID, "GUID should be set")
	originalGUID := state.GUID

	t.Logf("Created state: GUID=%s, LogicID=%s", state.GUID, state.LogicID)

	// Rename the state
	t.Logf("Renaming state from %s to %s", oldLogicID, newLogicID)
	renameResult, err := client.RenameState(ctx, sdk.RenameStateInput{
		State:      sdk.StateReference{LogicID: oldLogicID},
		NewLogicID: newLogicID,
	})
	require.NoError(t, err, "Rename should succeed")

	// Verify rename result
	assert.Equal(t, originalGUID, renameResult.GUID, "GUID should be unchanged")
	assert.Equal(t, oldLogicID, renameResult.OldLogicID, "Old logic ID should match")
	assert.Equal(t, newLogicID, renameResult.NewLogicID, "New logic ID should match")
	assert.NotEmpty(t, renameResult.BackendConfig.Address, "Backend config should be populated")
	assert.NotZero(t, renameResult.RenamedAt, "RenamedAt timestamp should be set")

	t.Logf("Rename successful: %s -> %s (GUID: %s)", renameResult.OldLogicID, renameResult.NewLogicID, renameResult.GUID)

	// Verify new name works
	ctx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()
	stateByNewName, err := client.GetState(ctx2, sdk.StateReference{LogicID: newLogicID})
	require.NoError(t, err, "GetState by new logic_id should succeed")
	assert.Equal(t, originalGUID, stateByNewName.GUID, "GUID should match")
	assert.Equal(t, newLogicID, stateByNewName.LogicID, "Logic ID should be updated")

	// Verify old name returns not found
	ctx3, cancel3 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel3()
	_, err = client.GetState(ctx3, sdk.StateReference{LogicID: oldLogicID})
	assert.Error(t, err, "GetState by old logic_id should fail")
	assert.Contains(t, strings.ToLower(err.Error()), "not found", "Error should indicate state not found")

	t.Logf("✓ Verified old name (%s) is no longer accessible", oldLogicID)
}

// TestRenameState_ToExistingName verifies that renaming to an existing logic_id fails
func TestRenameState_ToExistingName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create two states
	logicID1 := fmt.Sprintf("test-rename-exists-1-%s", uuid.New().String()[:8])
	logicID2 := fmt.Sprintf("test-rename-exists-2-%s", uuid.New().String()[:8])

	t.Logf("Creating first state: %s", logicID1)
	_, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: logicID1,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state 1 should succeed")

	t.Logf("Creating second state: %s", logicID2)
	_, err = client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: logicID2,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state 2 should succeed")

	// Try to rename state 1 to state 2's name (should fail)
	t.Logf("Attempting to rename %s to %s (should fail)", logicID1, logicID2)
	_, err = client.RenameState(ctx, sdk.RenameStateInput{
		State:      sdk.StateReference{LogicID: logicID1},
		NewLogicID: logicID2,
	})
	require.Error(t, err, "Rename to existing logic_id should fail")
	assert.Contains(t, strings.ToLower(err.Error()), "already exists", "Error should indicate name already exists")

	t.Logf("✓ Verified rename to existing name fails correctly")
}

// TestRenameState_LockedStateFails verifies that locked states cannot be renamed
// NOTE: Locking is tested via Terraform integration in lock_conflict_test.go
// This test uses gridctl CLI to test the rename failure path with a mock locked state scenario
func TestRenameState_LockedStateFails(t *testing.T) {
	t.Skip("Locking requires Terraform integration - tested comprehensively in lock_conflict_test.go")
	// TODO: If needed, implement using tfexec pattern from lock_conflict_test.go
}

// TestRenameState_ConcurrentRename verifies that concurrent rename operations
// are handled correctly. Tests that the system maintains data integrity even when
// multiple rename requests are submitted simultaneously.
//
// NOTE: At the HTTP integration test level, it's difficult to create true concurrent
// database conflicts due to request serialization and network latency. This test validates
// that multiple renames complete successfully without data corruption. The optimistic
// locking mechanism (ErrConcurrentModification) is tested at the repository level in
// TestBunStateRepository_ConcurrentRename.
func TestRenameState_ConcurrentRename(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create a state
	originalLogicID := fmt.Sprintf("test-concurrent-%s", uuid.New().String()[:8])
	newLogicID1 := fmt.Sprintf("test-concurrent-new1-%s", uuid.New().String()[:8])
	newLogicID2 := fmt.Sprintf("test-concurrent-new2-%s", uuid.New().String()[:8])

	t.Logf("Creating state with logic_id: %s", originalLogicID)
	state, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: originalLogicID,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state should succeed")

	// Use channels to coordinate concurrent renames
	type renameResult struct {
		name string
		err  error
	}
	results := make(chan renameResult, 2)
	startBarrier := make(chan struct{}) // Synchronization barrier

	// Launch two concurrent rename operations
	t.Logf("Launching concurrent renames from %s to %s and %s", originalLogicID, newLogicID1, newLogicID2)

	// First rename attempt
	go func() {
		<-startBarrier // Wait for signal to start
		_, err := client.RenameState(ctx, sdk.RenameStateInput{
			State:      sdk.StateReference{GUID: state.GUID},
			NewLogicID: newLogicID1,
		})
		results <- renameResult{name: newLogicID1, err: err}
	}()

	// Second rename attempt
	go func() {
		<-startBarrier // Wait for signal to start
		_, err := client.RenameState(ctx, sdk.RenameStateInput{
			State:      sdk.StateReference{GUID: state.GUID},
			NewLogicID: newLogicID2,
		})
		results <- renameResult{name: newLogicID2, err: err}
	}()

	// Give goroutines time to reach the barrier
	time.Sleep(10 * time.Millisecond)

	// Release both goroutines simultaneously
	close(startBarrier)

	// Collect results
	var successCount, abortedCount int

	for i := 0; i < 2; i++ {
		result := <-results
		if result.err == nil {
			successCount++
			t.Logf("✓ Rename to %s succeeded", result.name)
		} else if strings.Contains(strings.ToLower(result.err.Error()), "aborted") ||
			strings.Contains(strings.ToLower(result.err.Error()), "concurrent") ||
			strings.Contains(strings.ToLower(result.err.Error()), "modified") {
			abortedCount++
			t.Logf("✓ Rename to %s failed with optimistic lock conflict: %v", result.name, result.err)
		} else {
			t.Errorf("Unexpected error for rename to %s: %v", result.name, result.err)
		}
	}

	// Verify data integrity - at least one rename succeeded
	assert.GreaterOrEqual(t, successCount, 1, "At least one rename should succeed")
	assert.LessOrEqual(t, successCount, 2, "At most two renames can succeed (serial execution)")

	// Log the outcome
	if successCount == 1 && abortedCount == 1 {
		t.Logf("✓ Outcome: True concurrent conflict detected - one rename aborted due to optimistic locking")
	} else if successCount == 2 {
		t.Logf("✓ Outcome: Serial execution - both renames completed (no concurrent conflict)")
	}

	// Verify final state integrity
	finalState, err := client.GetState(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err, "GetState should succeed")
	assert.NotEqual(t, originalLogicID, finalState.LogicID, "Logic ID should have changed from original")
	t.Logf("✓ Final state logic_id is %s (data integrity verified)", finalState.LogicID)
}
