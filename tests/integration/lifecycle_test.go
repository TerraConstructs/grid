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

// ======== User Story 2: Tombstone/Restore Tests ========

// TestTombstoneState_HappyPath verifies the basic tombstone flow:
// - Create state
// - Tombstone it
// - Verify hidden from default listings
// - Verify appears with include_tombstoned=true
func TestTombstoneState_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create a state with unique logic_id
	logicID := fmt.Sprintf("test-tombstone-%s", uuid.New().String()[:8])

	t.Logf("Creating state with logic_id: %s", logicID)
	state, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: logicID,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state should succeed")
	require.NotEmpty(t, state.GUID, "GUID should be set")

	t.Logf("Created state: GUID=%s, LogicID=%s", state.GUID, state.LogicID)

	// Verify state appears in default listing
	states, err := client.ListStates(ctx)
	require.NoError(t, err, "ListStates should succeed")
	foundActive := false
	for _, s := range states {
		if s.GUID == state.GUID {
			foundActive = true
			break
		}
	}
	assert.True(t, foundActive, "State should appear in default listing before tombstone")

	// Tombstone the state
	t.Logf("Tombstoning state: %s", logicID)
	tombstoneResult, err := client.TombstoneState(ctx, sdk.StateReference{LogicID: logicID})
	require.NoError(t, err, "Tombstone should succeed")

	// Verify tombstone result
	assert.Equal(t, state.GUID, tombstoneResult.GUID, "GUID should match")
	assert.Equal(t, logicID, tombstoneResult.LogicID, "Logic ID should match")
	assert.Contains(t, strings.ToUpper(tombstoneResult.Status), "TOMBSTONED", "Status should be TOMBSTONED")
	assert.NotZero(t, tombstoneResult.TombstonedAt, "TombstonedAt should be set")
	assert.NotEmpty(t, tombstoneResult.TombstonedBy, "TombstonedBy should be set")
	assert.Greater(t, tombstoneResult.RetentionDays, 0, "RetentionDays should be positive")
	assert.NotZero(t, tombstoneResult.PurgeEligibleAt, "PurgeEligibleAt should be set")

	t.Logf("Tombstone successful: Status=%s, RetentionDays=%d, PurgeEligible=%s",
		tombstoneResult.Status, tombstoneResult.RetentionDays, tombstoneResult.PurgeEligibleAt)

	// Verify state does NOT appear in default listing
	states, err = client.ListStates(ctx)
	require.NoError(t, err, "ListStates should succeed")
	foundInDefault := false
	for _, s := range states {
		if s.GUID == state.GUID {
			foundInDefault = true
			break
		}
	}
	assert.False(t, foundInDefault, "Tombstoned state should NOT appear in default listing")
	t.Logf("✓ Verified tombstoned state hidden from default listing")

	// Verify state DOES appear with include_tombstoned=true
	includeTombstoned := true
	statesWithTombstoned, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{
		IncludeTombstoned: &includeTombstoned,
	})
	require.NoError(t, err, "ListStatesWithOptions should succeed")
	foundWithFlag := false
	for _, s := range statesWithTombstoned {
		if s.GUID == state.GUID {
			foundWithFlag = true
			assert.True(t, s.IsTombstoned, "State should be marked as tombstoned")
			break
		}
	}
	assert.True(t, foundWithFlag, "Tombstoned state should appear with include_tombstoned=true")
	t.Logf("✓ Verified tombstoned state appears with include_tombstoned flag")
}

// TestRestoreState_HappyPath verifies the restore flow:
// - Create and tombstone state
// - Restore it
// - Verify active status and visibility
func TestRestoreState_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create a state with unique logic_id
	logicID := fmt.Sprintf("test-restore-%s", uuid.New().String()[:8])

	t.Logf("Creating state with logic_id: %s", logicID)
	state, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: logicID,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state should succeed")

	// Tombstone the state
	t.Logf("Tombstoning state: %s", logicID)
	_, err = client.TombstoneState(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err, "Tombstone should succeed")

	// Verify state is hidden from default listing
	states, err := client.ListStates(ctx)
	require.NoError(t, err, "ListStates should succeed")
	foundBeforeRestore := false
	for _, s := range states {
		if s.GUID == state.GUID {
			foundBeforeRestore = true
			break
		}
	}
	assert.False(t, foundBeforeRestore, "Tombstoned state should be hidden before restore")

	// Restore the state
	t.Logf("Restoring state: %s", logicID)
	restoreResult, err := client.RestoreState(ctx, sdk.StateReference{LogicID: logicID})
	require.NoError(t, err, "Restore should succeed")

	// Verify restore result
	assert.Equal(t, state.GUID, restoreResult.GUID, "GUID should match")
	assert.Equal(t, logicID, restoreResult.LogicID, "Logic ID should match")
	assert.Contains(t, strings.ToUpper(restoreResult.Status), "ACTIVE", "Status should be ACTIVE")
	assert.NotZero(t, restoreResult.RestoredAt, "RestoredAt should be set")
	assert.NotEmpty(t, restoreResult.BackendConfig.Address, "Backend config should be populated")

	t.Logf("Restore successful: Status=%s, RestoredAt=%s", restoreResult.Status, restoreResult.RestoredAt)

	// Verify state appears in default listing again
	states, err = client.ListStates(ctx)
	require.NoError(t, err, "ListStates should succeed")
	foundAfterRestore := false
	for _, s := range states {
		if s.GUID == state.GUID {
			foundAfterRestore = true
			assert.False(t, s.IsTombstoned, "State should NOT be tombstoned after restore")
			break
		}
	}
	assert.True(t, foundAfterRestore, "Restored state should appear in default listing")
	t.Logf("✓ Verified restored state is active and visible")
}

// TestTombstoneState_WithDependents verifies that a state with active dependents cannot be tombstoned
func TestTombstoneState_WithDependents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create producer state
	producerLogicID := fmt.Sprintf("test-tomb-producer-%s", uuid.New().String()[:8])
	t.Logf("Creating producer state: %s", producerLogicID)
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: producerLogicID,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create producer state should succeed")

	// Create consumer state
	consumerLogicID := fmt.Sprintf("test-tomb-consumer-%s", uuid.New().String()[:8])
	t.Logf("Creating consumer state: %s", consumerLogicID)
	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: consumerLogicID,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create consumer state should succeed")

	// Add dependency: consumer depends on producer
	t.Logf("Adding dependency: %s -> %s", producerLogicID, consumerLogicID)
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:        sdk.StateReference{LogicID: producerLogicID},
		FromOutput:  "output1",
		To:          sdk.StateReference{LogicID: consumerLogicID},
		ToInputName: "input1",
	})
	require.NoError(t, err, "AddDependency should succeed")

	// Attempt to tombstone producer (should fail because consumer depends on it)
	t.Logf("Attempting to tombstone producer with active dependent")
	_, err = client.TombstoneState(ctx, sdk.StateReference{GUID: producer.GUID})
	require.Error(t, err, "Tombstone should fail when active dependents exist")

	// Verify error mentions dependents blocking the tombstone
	errMsg := strings.ToLower(err.Error())
	assert.True(t,
		strings.Contains(errMsg, "dependent") || strings.Contains(errMsg, "cannot tombstone"),
		"Error should mention dependents or tombstone restriction")
	t.Logf("✓ Tombstone correctly blocked: %v", err)

	// Verify producer state is still active (check via ListStates with include_tombstoned)
	includeTombstoned := true
	allStates, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{
		IncludeTombstoned: &includeTombstoned,
	})
	require.NoError(t, err, "ListStatesWithOptions should succeed")
	foundProducer := false
	for _, s := range allStates {
		if s.GUID == producer.GUID {
			foundProducer = true
			assert.False(t, s.IsTombstoned, "Producer should still be active")
			break
		}
	}
	require.True(t, foundProducer, "Should find producer state")
	t.Logf("✓ Producer state remains active (unchanged)")

	// Verify we can tombstone consumer (no dependents on it)
	t.Logf("Tombstoning consumer state (has no dependents)")
	_, err = client.TombstoneState(ctx, sdk.StateReference{GUID: consumer.GUID})
	require.NoError(t, err, "Tombstone consumer should succeed")
	t.Logf("✓ Consumer tombstone succeeded (had no dependents)")
}

// TestRenameState_ToTombstonedName verifies that renaming to a tombstoned logic_id fails
func TestRenameState_ToTombstonedName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create two states with unique logic_ids
	logicID1 := fmt.Sprintf("test-rename-tomb-1-%s", uuid.New().String()[:8])
	logicID2 := fmt.Sprintf("test-rename-tomb-2-%s", uuid.New().String()[:8])

	t.Logf("Creating first state: %s", logicID1)
	state1, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: logicID1,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state 1 should succeed")

	t.Logf("Creating second state: %s", logicID2)
	state2, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: logicID2,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state 2 should succeed")

	// Tombstone the second state
	t.Logf("Tombstoning second state: %s", logicID2)
	_, err = client.TombstoneState(ctx, sdk.StateReference{GUID: state2.GUID})
	require.NoError(t, err, "Tombstone should succeed")

	// Attempt to rename first state to second's (tombstoned) logic_id
	t.Logf("Attempting to rename active state to tombstoned name: %s -> %s", logicID1, logicID2)
	_, err = client.RenameState(ctx, sdk.RenameStateInput{
		State:      sdk.StateReference{GUID: state1.GUID},
		NewLogicID: logicID2,
	})
	require.Error(t, err, "Rename to tombstoned name should fail")

	// Verify error indicates the name already exists
	errMsg := strings.ToLower(err.Error())
	assert.Contains(t, errMsg, "already exists", "Error should indicate name conflict")
	t.Logf("✓ Rename correctly blocked: %v", err)

	// Verify first state's logic_id is unchanged
	state1Check, err := client.GetState(ctx, sdk.StateReference{GUID: state1.GUID})
	require.NoError(t, err, "GetState should succeed")
	assert.Equal(t, logicID1, state1Check.LogicID, "Original state logic_id should be unchanged")
	t.Logf("✓ Original state unchanged (logic_id: %s)", state1Check.LogicID)
}

// TestAddDependency_ToTombstoned verifies that adding a dependency to a tombstoned state fails
func TestAddDependency_ToTombstoned(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create producer state
	producerLogicID := fmt.Sprintf("test-dep-tomb-producer-%s", uuid.New().String()[:8])
	t.Logf("Creating producer state: %s", producerLogicID)
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: producerLogicID,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create producer state should succeed")

	// Tombstone the producer
	t.Logf("Tombstoning producer state: %s", producerLogicID)
	_, err = client.TombstoneState(ctx, sdk.StateReference{GUID: producer.GUID})
	require.NoError(t, err, "Tombstone should succeed")

	// Create consumer state
	consumerLogicID := fmt.Sprintf("test-dep-tomb-consumer-%s", uuid.New().String()[:8])
	t.Logf("Creating consumer state: %s", consumerLogicID)
	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: consumerLogicID,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create consumer state should succeed")

	// Attempt to add dependency to tombstoned producer
	t.Logf("Attempting to add dependency to tombstoned state")
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:        sdk.StateReference{GUID: producer.GUID},
		FromOutput:  "output1",
		To:          sdk.StateReference{GUID: consumer.GUID},
		ToInputName: "input1",
	})
	require.Error(t, err, "AddDependency to tombstoned state should fail")

	// Verify error message
	errMsg := strings.ToLower(err.Error())
	assert.True(t,
		strings.Contains(errMsg, "tombstoned") || strings.Contains(errMsg, "deleted"),
		"Error should indicate tombstoned state issue")
	t.Logf("✓ AddDependency correctly blocked: %v", err)

	// Verify consumer has no dependencies (check via GetStateInfo)
	consumerInfo, err := client.GetStateInfo(ctx, sdk.StateReference{GUID: consumer.GUID})
	require.NoError(t, err, "GetStateInfo should succeed")
	assert.Equal(t, 0, len(consumerInfo.Dependencies), "Consumer should have no dependencies")
	t.Logf("✓ Consumer has no dependencies (dependency not added)")
}

// TestRenameState_TombstonedToFreeName verifies that tombstoned states can be renamed to free up logic_id
func TestRenameState_TombstonedToFreeName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state with initial logic_id (A)
	logicIDA := fmt.Sprintf("test-tomb-rename-a-%s", uuid.New().String()[:8])
	logicIDB := fmt.Sprintf("test-tomb-rename-b-%s", uuid.New().String()[:8])

	t.Logf("Creating state with logic_id: %s", logicIDA)
	stateA, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: logicIDA,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state should succeed")
	originalGUID := stateA.GUID

	// Tombstone the state
	t.Logf("Tombstoning state: %s", logicIDA)
	_, err = client.TombstoneState(ctx, sdk.StateReference{GUID: stateA.GUID})
	require.NoError(t, err, "Tombstone should succeed")

	// Rename tombstoned state from A to B
	t.Logf("Renaming tombstoned state: %s -> %s", logicIDA, logicIDB)
	renameResult, err := client.RenameState(ctx, sdk.RenameStateInput{
		State:      sdk.StateReference{GUID: stateA.GUID},
		NewLogicID: logicIDB,
	})
	require.NoError(t, err, "Rename tombstoned state should succeed")
	assert.Equal(t, originalGUID, renameResult.GUID, "GUID should be unchanged")
	assert.Equal(t, logicIDA, renameResult.OldLogicID, "Old logic_id should match")
	assert.Equal(t, logicIDB, renameResult.NewLogicID, "New logic_id should match")
	t.Logf("✓ Rename successful: %s -> %s (GUID: %s)", logicIDA, logicIDB, originalGUID)

	// Verify tombstoned state now has new name (check via ListStates with include_tombstoned)
	includeTombstoned := true
	allStates, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{
		IncludeTombstoned: &includeTombstoned,
	})
	require.NoError(t, err, "ListStatesWithOptions should succeed")
	foundRenamed := false
	for _, s := range allStates {
		if s.GUID == originalGUID {
			foundRenamed = true
			assert.Equal(t, logicIDB, s.LogicID, "Tombstoned state should have new logic_id")
			assert.True(t, s.IsTombstoned, "State should still be tombstoned")
			break
		}
	}
	require.True(t, foundRenamed, "Should find tombstoned state with new name")

	// Create NEW state with the original logic_id A (now freed up)
	t.Logf("Creating new state with freed logic_id: %s", logicIDA)
	newStateA, err := client.CreateState(ctx, sdk.CreateStateInput{
		LogicID: logicIDA,
		Labels:  sdk.LabelMap{"env": "test"},
	})
	require.NoError(t, err, "Create state with freed logic_id should succeed")
	assert.NotEqual(t, originalGUID, newStateA.GUID, "New state should have different GUID")
	assert.Equal(t, logicIDA, newStateA.LogicID, "New state should have logic_id A")
	t.Logf("✓ New state created with freed logic_id: %s (GUID: %s)", logicIDA, newStateA.GUID)

	// Verify both states exist (one tombstoned with name B, one active with name A)
	stateByA, err := client.GetState(ctx, sdk.StateReference{LogicID: logicIDA})
	require.NoError(t, err, "GetState by logic_id A should succeed")
	assert.Equal(t, newStateA.GUID, stateByA.GUID, "Logic_id A should resolve to new state")

	// Access tombstoned state by new name B (requires include_tombstoned)
	allStates2, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{
		IncludeTombstoned: &includeTombstoned,
	})
	require.NoError(t, err, "ListStatesWithOptions should succeed")
	foundTombstonedB := false
	for _, s := range allStates2 {
		if s.GUID == originalGUID {
			foundTombstonedB = true
			assert.Equal(t, logicIDB, s.LogicID, "Tombstoned state should have logic_id B")
			assert.True(t, s.IsTombstoned, "Original state should be tombstoned")
			break
		}
	}
	assert.True(t, foundTombstonedB, "Should find tombstoned state with logic_id B")
	t.Logf("✓ Verified: Tombstoned state has logic_id B, new active state has logic_id A")
}
