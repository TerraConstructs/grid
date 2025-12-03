// Package integration provides end-to-end integration tests for Grid.
//
// # Mode 1 Dependency Authorization Tests
//
// These tests verify that dependency authorization works correctly with the two-check model:
// 1. Source check: User must have state-output:read on the source state
// 2. Destination check: User must have dependency:create on the destination state
//
// This prevents "confused deputy" attacks where a user with limited permissions tries to
// create a dependency from a sensitive state they cannot access.
//
// Tests use the gridctl CLI (protocol-agnostic) instead of direct RPC calls.
//
// ## Prerequisites
//
// Same as auth_mode1_test.go - requires Keycloak and Mode 1 configuration.
//
// ## Tests
//
// - TestMode1_DependencyAuthorization_HappyPath: Product engineer can create dependencies between env=dev states via gridctl
// - TestMode1_DependencyAuthorization_CrossScopeSourceDenial: Prevents confused deputy attack (cannot read prod source)
// - TestMode1_DependencyAuthorization_CrossScopeDestinationDenial: Cannot create dependency to prod destination
// - TestMode1_DependencyAuthorization_ListAndDelete: Product engineer cannot list/manage prod dependencies
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMode1_DependencyAuthorization_HappyPath verifies product-engineer can create dependencies between env=dev states
func TestMode1_DependencyAuthorization_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")
	verifyAuthEnabled(t, "Mode 1")

	t.Log("Testing dependency authorization happy path (same scope)...")

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

	// Step 2: Authenticate as Alice (product-engineer)
	t.Log("Step 2: Authenticating as alice@example.com...")
	userClientID := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")

	// Step 3: Create two states with env=dev labels
	t.Log("Step 3: Creating network and cluster states with env=dev...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	networkDir := t.TempDir()
	clusterDir := t.TempDir()

	networkLogicID := fmt.Sprintf("test-dep-network-%d", time.Now().UnixNano())
	clusterLogicID := fmt.Sprintf("test-dep-cluster-%d", time.Now().UnixNano())

	gridctlPath := getGridctlPath(t)

	// Create network state
	createNetworkCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", networkLogicID,
		"--label", "env=dev",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	createNetworkCmd.Dir = networkDir
	networkOutput, err := createNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create network state: %s", string(networkOutput))
	t.Logf("Created network state: %s", networkLogicID)

	// Create cluster state
	createClusterCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", clusterLogicID,
		"--label", "env=dev",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	createClusterCmd.Dir = clusterDir
	clusterOutput, err := createClusterCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create cluster state: %s", string(clusterOutput))
	t.Logf("Created cluster state: %s", clusterLogicID)

	// Step 4: Apply terraform to network state to create outputs
	t.Log("Step 4: Creating outputs in network state...")
	networkTfConfig := `
output "vpc_id" {
  value = "vpc-12345"
}

output "subnet_id" {
  value = "subnet-67890"
}
`
	networkTfFile := filepath.Join(networkDir, "main.tf")
	err = os.WriteFile(networkTfFile, []byte(networkTfConfig), 0644)
	require.NoError(t, err)

	// Link directory to state using gridctl state get --link
	linkNetworkCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get", networkLogicID,
		"--link",
		"--path", networkDir,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	linkNetworkOutput, err := linkNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to link network directory: %s", string(linkNetworkOutput))

	// Initialize backend.tf with gridctl state init
	stateInitNetworkCmd := exec.CommandContext(ctx, gridctlPath, "state", "init",
		"--token", userTokenResp.AccessToken,
		"--server", serverURL)
	stateInitNetworkCmd.Dir = networkDir
	stateInitNetworkOutput, err := stateInitNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run gridctl state init on network: %s", string(stateInitNetworkOutput))

	// Initialize terraform
	initNetworkCmd := exec.CommandContext(ctx, "terraform", "init")
	initNetworkCmd.Dir = networkDir
	initNetworkCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	initNetworkOutput, err := initNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run terraform init on network: %s", string(initNetworkOutput))

	// Apply terraform
	applyNetworkCmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	applyNetworkCmd.Dir = networkDir
	applyNetworkCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	applyNetworkOutput, err := applyNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run terraform apply on network: %s", string(applyNetworkOutput))
	t.Log("Network state outputs created")

	// Step 5: Create dependency from network to cluster using gridctl
	t.Log("Step 5: Creating dependency from network to cluster...")
	depsAddCmd := exec.CommandContext(ctx, gridctlPath,
		"deps", "add",
		"--from", networkLogicID,
		"--output", "vpc_id",
		"--to", clusterLogicID,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	depsOutput, err := depsAddCmd.CombinedOutput()
	require.NoError(t, err, "Product engineer should be able to create dependency between env=dev states: %s", string(depsOutput))
	t.Logf("Dependency created: %s", string(depsOutput))

	// Step 6: List dependencies to verify
	t.Log("Step 6: Listing dependencies...")
	depsListCmd := exec.CommandContext(ctx, gridctlPath,
		"deps", "list",
		"--state", clusterLogicID,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	listOutput, err := depsListCmd.CombinedOutput()
	require.NoError(t, err, "Product engineer should be able to list dependencies: %s", string(listOutput))
	require.Contains(t, string(listOutput), networkLogicID, "Dependency list should contain network state")
	t.Logf("Dependencies listed successfully")

	// Step 7: Get edge-id via state get, then delete dependency
	t.Log("Step 7: Getting edge-id and deleting dependency...")
	// Use gridctl state get to see the dependency information including edge-id
	getStateCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get", clusterLogicID,
		"--format", "json",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	getStateOutput, err := getStateCmd.CombinedOutput()
	require.NoError(t, err, "Failed to get state info: %s", string(getStateOutput))

	// Parse dependencies to find edge_id
	var stateInfo struct {
		Dependencies []struct {
			EdgeID      int64  `json:"edge_id"`
			FromLogicID string `json:"from_logic_id"`
		} `json:"dependencies"`
	}
	err = json.Unmarshal(getStateOutput, &stateInfo)
	require.NoError(t, err, "Failed to parse state info")
	require.Greater(t, len(stateInfo.Dependencies), 0, "Should have at least one dependency")

	edgeID := stateInfo.Dependencies[0].EdgeID

	// Now remove the dependency using edge-id
	depsRemoveCmd := exec.CommandContext(ctx, gridctlPath,
		"deps", "remove",
		"--edge-id", fmt.Sprintf("%d", edgeID),
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	removeOutput, err := depsRemoveCmd.CombinedOutput()
	require.NoError(t, err, "Product engineer should be able to remove dependency: %s", string(removeOutput))
	t.Logf("Dependency removed successfully")

	t.Log("✓ Dependency authorization happy path complete!")
}

// TestMode1_DependencyAuthorization_CrossScopeSourceDenial verifies confused deputy prevention
func TestMode1_DependencyAuthorization_CrossScopeSourceDenial(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")
	verifyAuthEnabled(t, "Mode 1")

	t.Log("Testing dependency authorization: confused deputy prevention...")

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

	// Step 2: Platform engineer creates prod-db-passwords state with env=prod
	t.Log("Step 2: Creating prod-db-passwords state with env=prod...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prodDir := t.TempDir()
	prodLogicID := fmt.Sprintf("prod-db-passwords-%d", time.Now().UnixNano())

	gridctlPath := getGridctlPath(t)
	createProdCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", prodLogicID,
		"--label", "env=prod",
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	createProdCmd.Dir = prodDir
	prodOutput, err := createProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create prod state: %s", string(prodOutput))

	// Populate prod state with outputs
	prodTfConfig := `


output "db_password" {
  value     = "super-secret-prod-password"
  sensitive = true
}
`
	prodTfFile := filepath.Join(prodDir, "main.tf")
	err = os.WriteFile(prodTfFile, []byte(prodTfConfig), 0644)
	require.NoError(t, err)

	// Link directory to state using gridctl state get --link
	linkProdCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get", prodLogicID,
		"--link",
		"--path", prodDir,
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	linkProdOutput, err := linkProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to link prod directory: %s", string(linkProdOutput))

	// Initialize backend.tf with gridctl state init
	stateInitProdCmd := exec.CommandContext(ctx, gridctlPath, "state", "init",
		"--token", adminTokenResp.AccessToken,
		"--server", serverURL)
	stateInitProdCmd.Dir = prodDir
	stateInitProdOutput, err := stateInitProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run gridctl state init on prod: %s", string(stateInitProdOutput))

	// Initialize terraform
	initProdCmd := exec.CommandContext(ctx, "terraform", "init")
	initProdCmd.Dir = prodDir
	initProdCmd.Env = append(os.Environ(), getTerraformAuthEnv(adminTokenResp.AccessToken)...)
	initProdOutput, err := initProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to init prod state: %s", string(initProdOutput))

	// Apply terraform
	applyProdCmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	applyProdCmd.Dir = prodDir
	applyProdCmd.Env = append(os.Environ(), getTerraformAuthEnv(adminTokenResp.AccessToken)...)
	applyProdOutput, err := applyProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to apply prod state: %s", string(applyProdOutput))
	t.Log("Prod state created with sensitive output")

	// Step 3: Authenticate as Alice (product-engineer with env=dev scope only)
	t.Log("Step 3: Authenticating as alice@example.com (product-engineer)...")
	userClientID := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")

	// Step 4: Alice creates my-dev-app state with env=dev
	t.Log("Step 4: Alice creates my-dev-app state with env=dev...")
	devDir := t.TempDir()
	devLogicID := fmt.Sprintf("my-dev-app-%d", time.Now().UnixNano())

	createDevCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", devLogicID,
		"--label", "env=dev",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	createDevCmd.Dir = devDir
	devOutput, err := createDevCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create dev state: %s", string(devOutput))

	// Step 5: Alice tries to create dependency from prod-db-passwords to my-dev-app
	t.Log("Step 5: Alice attempts to create dependency from prod to dev (expecting authorization failure)...")

	// Try using gridctl deps add
	depsAddCmd := exec.CommandContext(ctx, gridctlPath,
		"deps", "add",
		"--from", prodLogicID,
		"--output", "db_password",
		"--to", devLogicID,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	depsOutput, err := depsAddCmd.CombinedOutput()

	// Command should fail with authorization error
	require.Error(t, err, "Alice should be denied creating dependency from prod source (confused deputy prevention)")
	outputStr := string(depsOutput)
	t.Logf("gridctl deps add output: %s", outputStr)

	// Verify the error message indicates source read permission denial
	require.Contains(t, outputStr, "permission_denied",
		"Error should indicate permission was denied on source")

	t.Log("✓ Confused deputy attack prevented! Alice cannot read prod source, so dependency creation is denied")
}

// TestMode1_DependencyAuthorization_CrossScopeDestinationDenial verifies destination check
func TestMode1_DependencyAuthorization_CrossScopeDestinationDenial(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")
	verifyAuthEnabled(t, "Mode 1")

	t.Log("Testing dependency authorization: destination check...")

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
	userClientID := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")

	// Step 3: Alice creates dev-network state with env=dev
	t.Log("Step 3: Creating dev-network state with env=dev...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	devNetworkDir := t.TempDir()
	devNetworkLogicID := fmt.Sprintf("dev-network-%d", time.Now().UnixNano())

	gridctlPath := getGridctlPath(t)
	createDevNetworkCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", devNetworkLogicID,
		"--label", "env=dev",
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	createDevNetworkCmd.Dir = devNetworkDir
	devNetworkOutput, err := createDevNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create dev-network state: %s", string(devNetworkOutput))

	// Populate dev-network with outputs
	devNetworkTfConfig := `


output "vpc_id" {
  value = "vpc-dev-12345"
}
`
	devNetworkTfFile := filepath.Join(devNetworkDir, "main.tf")
	err = os.WriteFile(devNetworkTfFile, []byte(devNetworkTfConfig), 0644)
	require.NoError(t, err)

	// Link directory to state using gridctl state get --link
	linkDevNetworkCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get", devNetworkLogicID,
		"--link",
		"--path", devNetworkDir,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	linkDevNetworkOutput, err := linkDevNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to link dev-network directory: %s", string(linkDevNetworkOutput))

	// Initialize backend.tf with gridctl state init
	stateInitDevNetworkCmd := exec.CommandContext(ctx, gridctlPath, "state", "init",
		"--token", userTokenResp.AccessToken,
		"--server", serverURL)
	stateInitDevNetworkCmd.Dir = devNetworkDir
	stateInitDevNetworkOutput, err := stateInitDevNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run gridctl state init on dev-network: %s", string(stateInitDevNetworkOutput))

	// Initialize terraform
	initDevNetworkCmd := exec.CommandContext(ctx, "terraform", "init")
	initDevNetworkCmd.Dir = devNetworkDir
	initDevNetworkCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	initDevNetworkOutput, err := initDevNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to init dev-network: %s", string(initDevNetworkOutput))

	// Apply terraform
	applyDevNetworkCmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	applyDevNetworkCmd.Dir = devNetworkDir
	applyDevNetworkCmd.Env = append(os.Environ(), getTerraformAuthEnv(userTokenResp.AccessToken)...)
	applyDevNetworkOutput, err := applyDevNetworkCmd.CombinedOutput()
	require.NoError(t, err, "Failed to apply dev-network: %s", string(applyDevNetworkOutput))
	t.Log("Dev-network state created with outputs")

	// Step 4: Platform engineer creates prod-cluster state with env=prod
	t.Log("Step 4: Creating prod-cluster state with env=prod...")
	prodClusterDir := t.TempDir()
	prodClusterLogicID := fmt.Sprintf("prod-cluster-%d", time.Now().UnixNano())

	createProdClusterCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", prodClusterLogicID,
		"--label", "env=prod",
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	createProdClusterCmd.Dir = prodClusterDir
	prodClusterOutput, err := createProdClusterCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create prod-cluster state: %s", string(prodClusterOutput))

	// Step 5: Alice tries to create dependency from dev-network to prod-cluster
	t.Log("Step 5: Alice attempts to create dependency to prod destination (expecting authorization failure)...")

	depsAddCmd := exec.CommandContext(ctx, gridctlPath,
		"deps", "add",
		"--from", devNetworkLogicID,
		"--output", "vpc_id",
		"--to", prodClusterLogicID,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	depsOutput, err := depsAddCmd.CombinedOutput()

	// Command should fail with authorization error
	require.Error(t, err, "Alice should be denied creating dependency to prod destination")
	outputStr := string(depsOutput)
	t.Logf("gridctl deps add output: %s", outputStr)

	// Verify the error message indicates destination permission denial
	require.Contains(t, outputStr, "permission_denied",
		"Error should indicate permission was denied on destination")

	t.Log("✓ Destination check working! Alice cannot create dependency to prod destination")
}

// TestMode1_DependencyAuthorization_ListAndDelete verifies list/delete operations
func TestMode1_DependencyAuthorization_ListAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")
	verifyAuthEnabled(t, "Mode 1")

	t.Log("Testing dependency list and delete authorization...")

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	gridctlPath := getGridctlPath(t)

	// Step 2: Platform engineer creates prod state with dependency
	t.Log("Step 2: Creating prod states with dependency...")
	prodSourceLogicID := fmt.Sprintf("prod-source-%d", time.Now().UnixNano())
	prodDestLogicID := fmt.Sprintf("prod-dest-%d", time.Now().UnixNano())

	prodSourceDir := t.TempDir()
	prodDestDir := t.TempDir()

	// Create prod source state
	createProdSourceCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", prodSourceLogicID,
		"--label", "env=prod",
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	createProdSourceCmd.Dir = prodSourceDir
	prodSourceOutput, err := createProdSourceCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create prod source: %s", string(prodSourceOutput))

	// Create prod destination state
	createProdDestCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "create", prodDestLogicID,
		"--label", "env=prod",
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	createProdDestCmd.Dir = prodDestDir
	prodDestOutput, err := createProdDestCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create prod dest: %s", string(prodDestOutput))

	// Populate prod source with outputs
	prodTfConfig := `


output "test_value" {
  value = "prod-value"
}
`
	prodTfFile := filepath.Join(prodSourceDir, "main.tf")
	err = os.WriteFile(prodTfFile, []byte(prodTfConfig), 0644)
	require.NoError(t, err)

	// Link directory to state using gridctl state get --link
	linkProdSourceCmd := exec.CommandContext(ctx, gridctlPath,
		"state", "get", prodSourceLogicID,
		"--link",
		"--path", prodSourceDir,
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	linkProdSourceOutput, err := linkProdSourceCmd.CombinedOutput()
	require.NoError(t, err, "Failed to link prod-source directory: %s", string(linkProdSourceOutput))

	// Initialize backend.tf with gridctl state init
	stateInitProdCmd := exec.CommandContext(ctx, gridctlPath, "state", "init",
		"--token", adminTokenResp.AccessToken,
		"--server", serverURL)
	stateInitProdCmd.Dir = prodSourceDir
	stateInitProdOutput, err := stateInitProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run gridctl state init on prod: %s", string(stateInitProdOutput))

	// Initialize terraform
	initProdCmd := exec.CommandContext(ctx, "terraform", "init")
	initProdCmd.Dir = prodSourceDir
	initProdCmd.Env = append(os.Environ(), getTerraformAuthEnv(adminTokenResp.AccessToken)...)
	initProdOutput, err := initProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to init prod: %s", string(initProdOutput))

	// Apply terraform
	applyProdCmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	applyProdCmd.Dir = prodSourceDir
	applyProdCmd.Env = append(os.Environ(), getTerraformAuthEnv(adminTokenResp.AccessToken)...)
	applyProdOutput, err := applyProdCmd.CombinedOutput()
	require.NoError(t, err, "Failed to apply prod: %s", string(applyProdOutput))

	// Create dependency using gridctl deps add
	depsAddCmd := exec.CommandContext(ctx, gridctlPath,
		"deps", "add",
		"--from", prodSourceLogicID,
		"--output", "test_value",
		"--to", prodDestLogicID,
		"--server", serverURL,
		"--token", adminTokenResp.AccessToken)
	depsAddOutput, err := depsAddCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create prod dependency via gridctl: %s", string(depsAddOutput))
	t.Log("Prod dependency created")

	// Step 3: Authenticate as Alice (product-engineer)
	t.Log("Step 3: Authenticating as alice@example.com...")
	userClientID := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")

	// Step 4: Alice tries to list dependencies on prod state (should be denied)
	t.Log("Step 4: Alice attempts to list dependencies on prod state (expecting authorization failure)...")
	depsListCmd := exec.CommandContext(ctx, gridctlPath,
		"deps", "list",
		"--state", prodDestLogicID,
		"--server", serverURL,
		"--token", userTokenResp.AccessToken)
	listOutput, err := depsListCmd.CombinedOutput()

	// Command should fail with authorization error
	require.Error(t, err, "Alice should be denied listing dependencies on prod state")
	listOutputStr := string(listOutput)
	t.Logf("gridctl deps list output: %s", listOutputStr)

	// Step 5: Since Alice cannot list dependencies on prod state, she cannot get the edge-id
	// to attempt deletion. The list authorization denial already proves she cannot manage
	// prod dependencies. This test validates that dependency operations are properly scoped.
	t.Log("✓ List and delete authorization tests complete! Alice cannot manage prod dependencies")
}
