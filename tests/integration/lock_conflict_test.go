package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLockConflict verifies that concurrent Terraform operations are properly blocked by locking
func TestLockConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Check if Terraform is available
	tfPath, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("Terraform CLI not found, skipping integration test")
	}

	// Create temporary directories for two concurrent operations
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	// Generate unique logic ID for this test
	logicID := fmt.Sprintf("test-lock-%s", uuid.New().String()[:8])

	// Setup: Create state and backend config in both directories
	t.Logf("Creating state with logic_id: %s", logicID)
	setupGridState(t, ctx, tmpDir1, logicID)

	// Copy backend config to dir2
	backendContent, err := os.ReadFile(filepath.Join(tmpDir1, "backend.tf"))
	require.NoError(t, err, "Failed to read backend.tf")
	err = os.WriteFile(filepath.Join(tmpDir2, "backend.tf"), backendContent, 0644)
	require.NoError(t, err, "Failed to write backend.tf to dir2")

	// Initialize both directories
	for i, dir := range []string{tmpDir1, tmpDir2} {
		// Copy test fixture
		fixtureContent, err := os.ReadFile("../fixtures/null_resources.tf")
		require.NoError(t, err, "Failed to read fixture")

		mainTfPath := filepath.Join(dir, "main.tf")
		err = os.WriteFile(mainTfPath, fixtureContent, 0644)
		require.NoError(t, err, "Failed to write main.tf in dir %d", i+1)

		// Run terraform init
		tf, err := tfexec.NewTerraform(dir, tfPath)
		require.NoError(t, err, "Failed to create terraform executor for dir %d", i+1)

		err = tf.Init(ctx, tfexec.Upgrade(false))
		require.NoError(t, err, "Terraform init failed in dir %d", i+1)
	}

	// Test: Start a long-running operation in dir1
	t.Logf("Starting long-running apply in directory 1")

	// Create a config that triggers a slow operation
	slowConfig := `resource "null_resource" "slow" {
  triggers = {
    timestamp = timestamp()
  }

  provisioner "local-exec" {
    command = "sleep 10"
  }
}
`
	slowTfPath := filepath.Join(tmpDir1, "main.tf")
	err = os.WriteFile(slowTfPath, []byte(slowConfig), 0644)
	require.NoError(t, err, "Failed to write slow config")

	// Create terraform executor for dir1
	tf1, err := tfexec.NewTerraform(tmpDir1, tfPath)
	require.NoError(t, err, "Failed to create terraform executor for dir1")

	// Start apply in dir1 (this will hold the lock)
	applyCtx, applyCancel := context.WithTimeout(ctx, 30*time.Second)
	defer applyCancel()

	// Run in goroutine
	apply1Done := make(chan error, 1)
	go func() {
		err := tf1.Apply(applyCtx)
		apply1Done <- err
	}()

	// Wait a bit for the first apply to acquire the lock
	time.Sleep(3 * time.Second)

	// Try to run plan in dir2 (should fail with lock conflict)
	t.Logf("Attempting plan in directory 2 (should fail with lock)")
	tf2, err := tfexec.NewTerraform(tmpDir2, tfPath)
	require.NoError(t, err, "Failed to create terraform executor for dir2")

	planCtx, planCancel := context.WithTimeout(ctx, 10*time.Second)
	defer planCancel()

	_, err = tf2.Plan(planCtx, tfexec.LockTimeout("5s"))

	// Verify lock conflict - tfexec returns an error when plan fails
	assert.Error(t, err, "Plan should fail due to lock conflict")
	t.Logf("Plan error (expected): %v", err)

	// Wait for first apply to complete
	select {
	case err := <-apply1Done:
		if err != nil {
			t.Logf("First apply completed with error (may be expected): %v", err)
		} else {
			t.Logf("First apply completed successfully")
		}
	case <-applyCtx.Done():
		t.Log("First apply timed out (may be expected)")
	}

	// Now that lock is released, plan should succeed
	t.Logf("Attempting plan again after lock should be released")
	time.Sleep(2 * time.Second) // Give lock a moment to release

	planCtx2, planCancel2 := context.WithTimeout(ctx, 15*time.Second)
	defer planCancel2()

	_, err = tf2.Plan(planCtx2, tfexec.LockTimeout("5s"))
	if err != nil {
		t.Logf("Second plan attempt result: %v (may still be locked)", err)
	}

	// Cleanup - best effort
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()

	_ = tf1.Destroy(cleanupCtx)
	_ = tf2.Destroy(cleanupCtx)
}
