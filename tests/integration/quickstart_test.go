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

const (
	testTimeout = 60 * time.Second
)

// setupGridState creates a state via gridctl and returns the logic ID
func setupGridState(t *testing.T, ctx context.Context, workDir, logicID string) {
	gridctlPath := getGridctlPath(t)

	// Create state via gridctl
	t.Logf("Creating state with logic_id: %s", logicID)
	createCmd := exec.CommandContext(ctx, gridctlPath, "state", "create", logicID, "--server", serverURL)
	createCmd.Dir = workDir
	createOut, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create state: %s", string(createOut))

	// Initialize backend config
	t.Logf("Initializing backend config")
	initCmd := exec.CommandContext(ctx, gridctlPath, "state", "init", logicID, "--server", serverURL)
	initCmd.Dir = workDir
	initOut, err := initCmd.CombinedOutput()
	require.NoError(t, err, "Failed to init backend config: %s", string(initOut))

	// Verify backend.tf was created
	backendPath := filepath.Join(workDir, "backend.tf")
	require.FileExists(t, backendPath, "backend.tf should be created")
}

// TestQuickstartTerraform validates the complete workflow with Terraform CLI
func TestQuickstartTerraform(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Find terraform executable
	tfPath, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("Terraform CLI not found, skipping integration test")
	}

	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Generate unique logic ID for this test
	logicID := fmt.Sprintf("test-tf-%s", uuid.New().String()[:8])

	// Setup: Create state and backend config
	setupGridState(t, ctx, tmpDir, logicID)

	// Copy test fixture to temp directory
	fixtureContent, err := os.ReadFile("../fixtures/null_resources.tf")
	require.NoError(t, err, "Failed to read fixture")

	mainTfPath := filepath.Join(tmpDir, "main.tf")
	err = os.WriteFile(mainTfPath, fixtureContent, 0644)
	require.NoError(t, err, "Failed to write main.tf")

	// Create terraform executor
	tf, err := tfexec.NewTerraform(tmpDir, tfPath)
	require.NoError(t, err, "Failed to create terraform executor")

	// Step 1: Run terraform init
	t.Logf("Running terraform init")
	err = tf.Init(ctx, tfexec.Upgrade(false))
	require.NoError(t, err, "Terraform init failed")

	// Verify no local state file
	localStatePath := filepath.Join(tmpDir, "terraform.tfstate")
	assert.NoFileExists(t, localStatePath, "Local state file should not exist")

	// Step 2: Run terraform plan
	t.Logf("Running terraform plan")
	hasChanges, err := tf.Plan(ctx)
	require.NoError(t, err, "Terraform plan failed")
	assert.True(t, hasChanges, "Plan should show changes")

	// Step 3: Run terraform apply
	t.Logf("Running terraform apply")
	err = tf.Apply(ctx)
	require.NoError(t, err, "Terraform apply failed")

	// Verify state was persisted remotely
	gridctlPath := getGridctlPath(t)
	listCmd := exec.CommandContext(ctx, gridctlPath, "state", "list", "--server", serverURL)
	listOut, err := listCmd.CombinedOutput()
	require.NoError(t, err, "Failed to list states: %s", string(listOut))
	assert.Contains(t, string(listOut), logicID, "State should appear in list")

	// Verify state content
	state, err := tf.Show(ctx)
	require.NoError(t, err, "Failed to show state")
	assert.NotNil(t, state, "State should exist")
	assert.NotNil(t, state.Values, "State values should exist")

	// Step 4: Modify resources and plan again
	t.Logf("Modifying resources")
	modifiedContent := `resource "null_resource" "example" {
  triggers = {
    timestamp = "2025-10-01T00:00:00Z"
  }
}

resource "null_resource" "another" {
  triggers = {
    value = "updated"
  }
}
`
	err = os.WriteFile(mainTfPath, []byte(modifiedContent), 0644)
	require.NoError(t, err, "Failed to write modified main.tf")

	hasChanges2, err := tf.Plan(ctx)
	require.NoError(t, err, "Terraform plan (2nd) failed")
	assert.True(t, hasChanges2, "Plan should detect changes")

	// Step 5: Apply changes
	t.Logf("Applying changes")
	err = tf.Apply(ctx)
	require.NoError(t, err, "Terraform apply (2nd) failed")

	// Cleanup: Run terraform destroy
	t.Logf("Running terraform destroy")
	err = tf.Destroy(ctx)
	require.NoError(t, err, "Terraform destroy failed")
}

// TestQuickstartOpenTofu validates the complete workflow with OpenTofu CLI
func TestQuickstartOpenTofu(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Find tofu executable
	tofuPath, err := exec.LookPath("tofu")
	if err != nil {
		t.Skip("OpenTofu CLI not found, skipping integration test")
	}

	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Generate unique logic ID for this test
	logicID := fmt.Sprintf("test-tofu-%s", uuid.New().String()[:8])

	// Setup: Create state and backend config
	setupGridState(t, ctx, tmpDir, logicID)

	// Copy test fixture to temp directory
	fixtureContent, err := os.ReadFile("../fixtures/null_resources.tf")
	require.NoError(t, err, "Failed to read fixture")

	mainTfPath := filepath.Join(tmpDir, "main.tf")
	err = os.WriteFile(mainTfPath, fixtureContent, 0644)
	require.NoError(t, err, "Failed to write main.tf")

	// Create OpenTofu executor (tfexec works with OpenTofu too)
	tofu, err := tfexec.NewTerraform(tmpDir, tofuPath)
	require.NoError(t, err, "Failed to create tofu executor")

	// Run tofu init
	t.Logf("Running tofu init")
	err = tofu.Init(ctx, tfexec.Upgrade(false))
	require.NoError(t, err, "OpenTofu init failed")

	// Run tofu apply
	t.Logf("Running tofu apply")
	err = tofu.Apply(ctx)
	require.NoError(t, err, "OpenTofu apply failed")

	// Verify state was persisted remotely
	gridctlPath := getGridctlPath(t)
	listCmd := exec.CommandContext(ctx, gridctlPath, "state", "list", "--server", serverURL)
	listOut, err := listCmd.CombinedOutput()
	require.NoError(t, err, "Failed to list states: %s", string(listOut))
	assert.Contains(t, string(listOut), logicID, "State should appear in list")

	// Cleanup
	t.Logf("Running tofu destroy")
	_ = tofu.Destroy(ctx)
}
