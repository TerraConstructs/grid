// Package integration provides end-to-end integration tests for Grid.
//
// # Mode 1 State Output Authorization Tests
//
// These tests verify that state-output authorization works correctly with label-based
// access control. Product engineers with env=dev scope should only be able to list/read
// outputs from states with env=dev labels.
//
// Tests use the gridctl CLI (protocol-agnostic) instead of direct RPC calls.
//
// ## Prerequisites
//
// Same as auth_mode1_test.go - requires Keycloak and Mode 1 configuration.
//
// ## Tests
//
// - TestMode1_StateOutputAuthorization_HappyPath: Product engineer can view outputs from env=dev states via gridctl
// - TestMode1_StateOutputAuthorization_CrossScopeDenial: Product engineer cannot access env=prod state info
// - TestMode1_StateOutputAuthorization_WriteViaTerraform: Product engineer can trigger output updates via terraform apply
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMode1_StateOutputAuthorization_HappyPath verifies product-engineer can list/read outputs from env=dev states
func TestMode1_StateOutputAuthorization_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")
	verifyAuthEnabled(t, "Mode 1")

	t.Log("Testing state-output authorization happy path (env=dev)...")

	// Step 1: Setup - Map product-engineers group to product-engineer role
	t.Log("Step 1: Setting up RBAC...")
	setupKeycloakForGroupTests(t)

	testClientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	testClientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")
	if testClientID == "" || testClientSecret == "" {
		t.Skip("MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET must be set")
	}

	adminTokenResp := authenticateWithKeycloak(t, testClientID, testClientSecret)
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "product-engineers", "product-engineer")

	// Step 2: Authenticate as Alice (product-engineer with env=dev scope)
	t.Log("Step 2: Authenticating as alice@example.com...")
	userClientID := os.Getenv("EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")

	// Step 3: Create a state with env=dev label using gridctl
	t.Log("Step 3: Creating state with env=dev label via gridctl...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testDir := t.TempDir()
	logicID := fmt.Sprintf("test-output-happy-%d", time.Now().UnixNano())

	gridctlPath := getGridctlPath(t)
	createCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", logicID,
		"--label", "env=dev",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	createCmd.Dir = testDir
	output, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create state: %s", string(output))
	t.Logf("Created state: %s", string(output))

	// Parse GUID from output
	var guid string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Created state:") {
			guid = strings.TrimSpace(strings.TrimPrefix(line, "Created state:"))
			break
		}
	}
	require.NotEmpty(t, guid, "Failed to parse GUID from output")

	// Step 4: Create a simple Terraform config and apply it to generate outputs
	t.Log("Step 4: Running terraform apply to generate outputs...")
	tfConfig := `
resource "null_resource" "test" {
  triggers = {
    timestamp = timestamp()
  }
}

output "test_output" {
  value = "test-value-${null_resource.test.id}"
}

output "test_output_2" {
  value = "another-value"
}
`
	tfFile := filepath.Join(testDir, "main.tf")
	err = os.WriteFile(tfFile, []byte(tfConfig), 0644)
	require.NoError(t, err, "Failed to write Terraform config")

	// Initialize backend.tf file with gridctl state init
	stateInitCmd := exec.CommandContext(ctx, gridctlPath, "state", "init",
		"--token", userTokenResp.AccessToken,
		"--server", serverURL)
	stateInitCmd.Dir = testDir
	stateInitOutput, err := stateInitCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run gridctl state init: %s", string(stateInitOutput))

	// Initialize terraform
	initCmd := exec.CommandContext(ctx, "terraform", "init")
	initCmd.Dir = testDir
	initCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	initOutput, err := initCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run terraform init: %s", string(initOutput))

	// Apply terraform to generate outputs
	applyCmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	applyCmd.Dir = testDir
	applyCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	applyOutput, err := applyCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run terraform apply: %s", string(applyOutput))
	t.Logf("Terraform apply completed")

	// Step 5: List state outputs via gridctl state get
	t.Log("Step 5: Listing state outputs via gridctl state get...")
	getStateCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get", logicID,
		"--format", "json",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	getStateCmd.Dir = testDir
	getStateOutput, err := getStateCmd.CombinedOutput()
	require.NoError(t, err, "Product engineer should be able to get state info for env=dev state: %s", string(getStateOutput))

	// Parse JSON response to verify outputs exist
	var stateInfo struct {
		Outputs []struct {
			Key       string `json:"key"`
			Sensitive bool   `json:"sensitive"`
		} `json:"outputs"`
	}
	err = json.Unmarshal(getStateOutput, &stateInfo)
	require.NoError(t, err, "Failed to parse gridctl state get JSON output")
	require.Equal(t, 2, len(stateInfo.Outputs), "Should have exactly 2 outputs")

	// Verify expected output keys exist
	outputKeys := make(map[string]bool)
	for _, out := range stateInfo.Outputs {
		outputKeys[out.Key] = true
	}
	require.True(t, outputKeys["test_output"], "Should have test_output")
	require.True(t, outputKeys["test_output_2"], "Should have test_output_2")
	t.Logf("✓ Found expected outputs: test_output, test_output_2")

	t.Log("✓ State output authorization happy path complete!")
}

// TestMode1_StateOutputAuthorization_CrossScopeDenial verifies product-engineer cannot access env=prod outputs
func TestMode1_StateOutputAuthorization_CrossScopeDenial(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")
	verifyAuthEnabled(t, "Mode 1")

	t.Log("Testing state-output authorization cross-scope denial (env=prod)...")

	// Step 1: Setup - Map groups to roles
	t.Log("Step 1: Setting up RBAC...")
	setupKeycloakForGroupTests(t)

	testClientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	testClientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")
	if testClientID == "" || testClientSecret == "" {
		t.Skip("MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET must be set")
	}

	adminTokenResp := authenticateWithKeycloak(t, testClientID, testClientSecret)
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "product-engineers", "product-engineer")
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "platform-engineers", "platform-engineer")

	// Step 2: Platform engineer creates a state with env=prod label
	t.Log("Step 2: Creating env=prod state as platform engineer...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testDir := t.TempDir()
	logicID := fmt.Sprintf("test-output-prod-%d", time.Now().UnixNano())

	// Use admin token (platform-engineer role) to create prod state
	gridctlPath := getGridctlPath(t)
	createCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", logicID,
		"--label", "env=prod",
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	createCmd.Dir = testDir
	output, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create prod state: %s", string(output))
	t.Logf("Created prod state: %s", string(output))

	// Step 3: Populate state outputs using platform engineer credentials
	t.Log("Step 3: Populating outputs for prod state...")
	tfConfig := `


output "secret_value" {
  value = "production-secret"
}
`
	tfFile := filepath.Join(testDir, "main.tf")
	err = os.WriteFile(tfFile, []byte(tfConfig), 0644)
	require.NoError(t, err)

	// Initialize backend.tf with gridctl state init
	stateInitCmd := exec.CommandContext(ctx, gridctlPath, "state", "init",
		"--token", adminTokenResp.AccessToken,
		"--server", serverURL)
	stateInitCmd.Dir = testDir
	stateInitOutput, err := stateInitCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run gridctl state init: %s", string(stateInitOutput))

	// Initialize terraform
	initCmd := exec.CommandContext(ctx, "terraform", "init")
	initCmd.Dir = testDir
	initCmd.Env = append(os.Environ(), getTerraformAuthEnv(adminTokenResp.AccessToken)...)
	initOutput, err := initCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run terraform init: %s", string(initOutput))

	// Apply terraform
	applyCmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	applyCmd.Dir = testDir
	applyCmd.Env = append(os.Environ(), getTerraformAuthEnv(adminTokenResp.AccessToken)...)
	applyOutput, err := applyCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run terraform apply: %s", string(applyOutput))
	t.Log("Prod state populated with outputs")

	// Step 4: Authenticate as Alice (product-engineer with env=dev scope only)
	t.Log("Step 4: Authenticating as alice@example.com (product-engineer)...")
	userClientID := os.Getenv("EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")

	// Step 5: Try to get state info from prod state - should be denied
	t.Log("Step 5: Attempting to get state info from env=prod state (expecting authorization failure)...")
	getStateCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get", logicID,
		"--format", "json",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	getStateCmd.Dir = testDir
	getStateOutput, err := getStateCmd.CombinedOutput()

	// Command should fail with authorization error
	require.Error(t, err, "Product engineer should be denied access to env=prod state")
	outputStr := string(getStateOutput)
	t.Logf("gridctl state get output: %s", outputStr)

	// Verify it's an authorization failure (not a different error)
	require.Contains(t, outputStr, "permission_denied",
		"Should receive permission_denied error when trying to access env=prod state")

	t.Log("✓ Cross-scope denial test complete! Product engineer correctly denied access to env=prod outputs")
}

// TestMode1_StateOutputAuthorization_WriteViaTerraform verifies write authorization via terraform apply
func TestMode1_StateOutputAuthorization_WriteViaTerraform(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")
	verifyAuthEnabled(t, "Mode 1")

	t.Log("Testing state-output write authorization via terraform apply...")

	// Step 1: Setup RBAC
	t.Log("Step 1: Setting up RBAC...")
	setupKeycloakForGroupTests(t)

	testClientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	testClientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")
	if testClientID == "" || testClientSecret == "" {
		t.Skip("MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET must be set")
	}

	adminTokenResp := authenticateWithKeycloak(t, testClientID, testClientSecret)
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "product-engineers", "product-engineer")
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "platform-engineers", "platform-engineer")

	// Step 2: Authenticate as Alice (product-engineer)
	t.Log("Step 2: Authenticating as alice@example.com...")
	userClientID := os.Getenv("EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")

	// Step 3: Create env=dev state (should succeed)
	t.Log("Step 3: Creating env=dev state...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	devDir := t.TempDir()
	devLogicID := fmt.Sprintf("test-output-write-dev-%d", time.Now().UnixNano())

	gridctlPath := getGridctlPath(t)
	createCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", devLogicID,
		"--label", "env=dev",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	createCmd.Dir = devDir
	output, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create env=dev state: %s", string(output))

	// Step 4: Apply terraform to env=dev state (should succeed)
	t.Log("Step 4: Running terraform apply on env=dev state (expecting success)...")
	tfConfig := `


output "test" {
  value = "success"
}
`
	tfFile := filepath.Join(devDir, "main.tf")
	err = os.WriteFile(tfFile, []byte(tfConfig), 0644)
	require.NoError(t, err)

	// Initialize backend.tf with gridctl state init
	stateInitCmd := exec.CommandContext(ctx, gridctlPath, "state", "init",
		"--token", adminTokenResp.AccessToken,
		"--server", serverURL)
	stateInitCmd.Dir = devDir
	stateInitOutput, err := stateInitCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run gridctl state init: %s", string(stateInitOutput))

	// Initialize terraform
	initCmd := exec.CommandContext(ctx, "terraform", "init")
	initCmd.Dir = devDir
	initCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	initOutput, err := initCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run terraform init on env=dev: %s", string(initOutput))

	// Apply terraform
	applyCmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	applyCmd.Dir = devDir
	applyCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	applyOutput, err := applyCmd.CombinedOutput()
	require.NoError(t, err, "Product engineer should be able to update outputs on env=dev state: %s", string(applyOutput))
	t.Log("✓ Terraform apply succeeded on env=dev state")

	// Step 5: Create env=prod state as platform engineer
	t.Log("Step 5: Creating env=prod state as platform engineer...")
	prodDir := t.TempDir()
	prodLogicID := fmt.Sprintf("test-output-write-prod-%d", time.Now().UnixNano())

	createProdCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", prodLogicID,
		"--label", "env=prod",
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	createProdCmd.Dir = prodDir
	prodOutput, err := createProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create env=prod state: %s", string(prodOutput))

	// Step 6: Try to apply terraform to env=prod state as product engineer (should fail)
	t.Log("Step 6: Attempting terraform apply on env=prod state as product engineer (expecting failure)...")
	tfFileProd := filepath.Join(prodDir, "main.tf")
	err = os.WriteFile(tfFileProd, []byte(tfConfig), 0644)
	require.NoError(t, err)

	// Initialize backend.tf with admin credentials
	stateInitProdCmd := exec.CommandContext(ctx, gridctlPath, "state", "init",
		"--token", adminTokenResp.AccessToken,
		"--server", serverURL)
	stateInitProdCmd.Dir = prodDir
	stateInitProdOutput, err := stateInitProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run gridctl state init on env=prod: %s", string(stateInitProdOutput))

	// Initialize terraform with admin credentials
	initProdCmd := exec.CommandContext(ctx, "terraform", "init")
	initProdCmd.Dir = prodDir
	initProdCmd.Env = append(os.Environ(), getTerraformAuthEnv(adminTokenResp.AccessToken)...)
	initProdOutput, err := initProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run terraform init on env=prod: %s", string(initProdOutput))

	// Now try to apply with product engineer credentials (should fail)
	// TODO: Fix issue with passing through -auto-approve in gridctl tf apply
	applyProdCmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	applyProdCmd.Dir = prodDir
	applyProdCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	applyProdOutput, err := applyProdCmd.CombinedOutput()

	t.Logf("Terraform apply on env=prod output: %s", string(applyProdOutput))
	require.Error(t, err, "Product engineer should NOT be able to update outputs on env=prod state")
	require.Contains(t, string(applyProdOutput), "invalid auth",
		"Should receive invalid auth error when trying to update env=prod state")
	t.Log("✓ Terraform apply correctly denied on env=prod state")

	t.Log("✓ Write authorization via terraform apply test complete!")
}

// TestMode1_OutputSchemaAuthorization tests schema operations with RBAC
func TestMode1_OutputSchemaAuthorization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")
	verifyAuthEnabled(t, "Mode 1")

	t.Log("Testing output schema authorization...")

	// Setup RBAC
	setupKeycloakForGroupTests(t)

	testClientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	testClientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")
	if testClientID == "" || testClientSecret == "" {
		t.Skip("MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET must be set")
	}

	adminTokenResp := authenticateWithKeycloak(t, testClientID, testClientSecret)
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "product-engineers", "product-engineer")

	// Authenticate as product engineer (env=dev scope)
	userClientID := os.Getenv("EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logicID := fmt.Sprintf("test-schema-auth-%d", time.Now().UnixNano())

	// Create state with env=dev label
	gridctlPath := getGridctlPath(t)
	createCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", logicID,
		"--label", "env=dev",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	output, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create state: %s", string(output))

	// Get schema file path
	schemaPath, err := filepath.Abs(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)

	// Product engineer SHOULD be able to set schema on env=dev state
	setSchemaCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "set-output-schema",
		"--logic-id", logicID,
		"--output-key", "vpc_id",
		"--schema-file", schemaPath,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	setOutput, err := setSchemaCmd.CombinedOutput()
	require.NoError(t, err, "Product engineer should be able to set schema on env=dev state: %s", string(setOutput))
	require.Contains(t, string(setOutput), "Set schema for output 'vpc_id'")
	t.Log("✓ Product engineer can set output schema on env=dev state")

	// Product engineer SHOULD be able to get schema from env=dev state
	getSchemaCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get-output-schema",
		"--logic-id", logicID,
		"--output-key", "vpc_id",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	getOutput, err := getSchemaCmd.CombinedOutput()
	require.NoError(t, err, "Product engineer should be able to get schema from env=dev state: %s", string(getOutput))
	
	// Verify schema content
	schemaBytes, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	
	var expectedSchema, actualSchema map[string]interface{}
	err = json.Unmarshal(schemaBytes, &expectedSchema)
	require.NoError(t, err)
	err = json.Unmarshal(getOutput, &actualSchema)
	require.NoError(t, err)
	require.Equal(t, expectedSchema, actualSchema, "Retrieved schema should match")
	t.Log("✓ Product engineer can get output schema from env=dev state")

	// Now create env=prod state (using admin)
	prodLogicID := fmt.Sprintf("test-schema-auth-prod-%d", time.Now().UnixNano())
	createProdCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", prodLogicID,
		"--label", "env=prod",
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	prodOutput, err := createProdCmd.CombinedOutput()
	require.NoError(t, err, "Admin should be able to create env=prod state: %s", string(prodOutput))

	// Admin sets schema on env=prod state
	setProdSchemaCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "set-output-schema",
		"--logic-id", prodLogicID,
		"--output-key", "vpc_id",
		"--schema-file", schemaPath,
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	setProdOutput, err := setProdSchemaCmd.CombinedOutput()
	require.NoError(t, err, "Admin should be able to set schema on env=prod state: %s", string(setProdOutput))

	// Product engineer should NOT be able to write schema to env=prod state
	setUnauthorizedCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "set-output-schema",
		"--logic-id", prodLogicID,
		"--output-key", "vpc_cidr",
		"--schema-file", schemaPath,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	setUnauthorizedOutput, err := setUnauthorizedCmd.CombinedOutput()
	require.Error(t, err, "Product engineer should NOT be able to set schema on env=prod state")
	t.Logf("Expected error output: %s", string(setUnauthorizedOutput))
	t.Log("✓ Product engineer correctly denied write access to env=prod state schema")

	// Product engineer should NOT be able to read schema from env=prod state
	getUnauthorizedCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get-output-schema",
		"--logic-id", prodLogicID,
		"--output-key", "vpc_id",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	getUnauthorizedOutput, err := getUnauthorizedCmd.CombinedOutput()
	require.Error(t, err, "Product engineer should NOT be able to get schema from env=prod state")
	t.Logf("Expected error output: %s", string(getUnauthorizedOutput))
	t.Log("✓ Product engineer correctly denied read access to env=prod state schema")

	t.Log("✓ Output schema authorization tests complete!")
}
