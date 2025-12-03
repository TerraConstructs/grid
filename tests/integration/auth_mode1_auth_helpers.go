package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ===============================================
// Authentication Helpers
// ===============================================

// keycloakTokenResponse represents the response from Keycloak's token endpoint
type keycloakTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// authenticateWithKeycloak performs client credentials flow against Keycloak
func authenticateWithKeycloak(t *testing.T, clientID, clientSecret string) *keycloakTokenResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenURL := fmt.Sprintf("%s%s", keycloakBaseURL, keycloakTokenPath)

	// Prepare form data
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	// Request token for the Grid API resource server (same as gridapi's GRID_OIDC_EXTERNAL_IDP_CLIENT_ID)
	resourceServerAudience := os.Getenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
	if resourceServerAudience != "" {
		data.Set("audience", resourceServerAudience)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(data.Encode()))
	require.NoError(t, err, "Failed to create token request")

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Keycloak token endpoint")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read token response")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Keycloak token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp keycloakTokenResponse
	err = json.Unmarshal(body, &tokenResp)
	require.NoError(t, err, "Failed to parse token response")

	return &tokenResp
}

// authenticateUserWithPassword performs password grant (direct access grants) against Keycloak
func authenticateUserWithPassword(t *testing.T, clientID, clientSecret, username, password string) *keycloakTokenResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenURL := fmt.Sprintf("%s%s", keycloakBaseURL, keycloakTokenPath)

	// Prepare form data for password grant
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("username", username)
	data.Set("password", password)
	data.Set("scope", "openid profile email")

	// Request token for the Grid API resource server (same as gridapi's GRID_OIDC_EXTERNAL_IDP_CLIENT_ID)
	// CRITICAL: gridapi validates tokens with RequiredAudience=GRID_OIDC_EXTERNAL_IDP_CLIENT_ID (see jwt.go:94, jwt.go:122)
	// Without this, user tokens will be rejected with 401 even if signature/issuer are valid
	if clientID != "" {
		data.Set("audience", clientID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(data.Encode()))
	require.NoError(t, err, "Failed to create token request")

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Keycloak token endpoint")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read token response")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Keycloak token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp keycloakTokenResponse
	err = json.Unmarshal(body, &tokenResp)
	require.NoError(t, err, "Failed to parse token response")

	return &tokenResp
}

// setupKeycloakForGroupTests creates dynamic test data for group-based testing
//
// Static configuration (groups, mappers, service account membership) is defined in
// tests/fixtures/realm-export.json and imported on Keycloak startup.
//
// This function only handles test-specific dynamic data:
//   - Creating test user alice@example.com
//   - Setting alice's password
//   - Adding alice to the product-engineers group
func setupKeycloakForGroupTests(t *testing.T) {
	t.Helper()
	t.Log("Setting up Keycloak: test user alice@example.com...")

	// Create alice user, set password, and add to product-engineers group
	// Static config (groups, mappers) comes from realm-export.json
	setupScript := `set -e

# Login as admin
/opt/keycloak/bin/kcadm.sh config credentials \
  --server http://localhost:8080 --realm master --user admin --password admin

# Create test user alice@example.com (idempotent)
/opt/keycloak/bin/kcadm.sh create users -r grid \
  -s username=alice@example.com \
  -s email=alice@example.com \
  -s enabled=true || true

# Set password for alice
/opt/keycloak/bin/kcadm.sh set-password -r grid \
  --username alice@example.com --new-password "test123" --temporary=false

# Add alice to product-engineers group
USER_ID=$(/opt/keycloak/bin/kcadm.sh get users -r grid -q username=alice@example.com --fields id --format csv --noquotes | tail -n1)
GROUP_ID=$(/opt/keycloak/bin/kcadm.sh get groups -r grid --fields id,name --format csv --noquotes | grep product-engineers | cut -d, -f1)
/opt/keycloak/bin/kcadm.sh update users/$USER_ID/groups/$GROUP_ID -r grid -n || true

echo "✓ Test user setup complete"
`

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Logf("Running Keycloak test user setup via docker compose...")

	// Execute setup script via docker compose exec
	dockerCmd := exec.CommandContext(ctx, "docker", "compose", "exec", "-T", "keycloak", "bash", "-c", setupScript)
	output, err := dockerCmd.CombinedOutput()

	if err != nil {
		t.Logf("Keycloak test user setup warning: %v", err)
		t.Logf("Output: %s", string(output))
		t.Log("Note: Test user setup may have failed. Some tests may be skipped.")
		// Don't fail the test - let individual tests decide if they can continue
	} else {
		t.Logf("Keycloak test user setup output: %s", string(output))
	}
}

// addUserToKeycloakGroup adds an existing Keycloak user to a group.
// This is useful for testing multiple group memberships and union semantics.
func addUserToKeycloakGroup(t *testing.T, username, groupName string) {
	t.Helper()

	addToGroupScript := fmt.Sprintf(`set -e

# Login as admin
/opt/keycloak/bin/kcadm.sh config credentials \
  --server http://localhost:8080 --realm master --user admin --password admin

# Get user ID
USER_ID=$(/opt/keycloak/bin/kcadm.sh get users -r grid -q username=%s --fields id --format csv --noquotes | tail -n1)

# Get group ID
GROUP_ID=$(/opt/keycloak/bin/kcadm.sh get groups -r grid --fields id,name --format csv --noquotes | grep %s | cut -d, -f1)

# Add user to group (idempotent - won't fail if already member)
/opt/keycloak/bin/kcadm.sh update users/$USER_ID/groups/$GROUP_ID -r grid -n || true

echo "✓ Added user %s to group %s"
`, username, groupName, username, groupName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dockerCmd := exec.CommandContext(ctx, "docker", "compose", "exec", "-T", "keycloak", "bash", "-c", addToGroupScript)
	output, err := dockerCmd.CombinedOutput()

	if err != nil {
		t.Logf("Warning: Failed to add user to group: %v", err)
		t.Logf("Output: %s", string(output))
	} else {
		t.Logf("Added user %s to group %s: %s", username, groupName, string(output))
	}
}

// removeUserFromKeycloakGroup removes a user from a Keycloak group.
// This is useful for test isolation to ensure users have specific group memberships.
func removeUserFromKeycloakGroup(t *testing.T, username, groupName string) {
	t.Helper()

	removeFromGroupScript := fmt.Sprintf(`set -e

# Login as admin
/opt/keycloak/bin/kcadm.sh config credentials \
  --server http://localhost:8080 --realm master --user admin --password admin

# Get user ID
USER_ID=$(/opt/keycloak/bin/kcadm.sh get users -r grid -q username=%s --fields id --format csv --noquotes | tail -n1)

# Get group ID
GROUP_ID=$(/opt/keycloak/bin/kcadm.sh get groups -r grid --fields id,name --format csv --noquotes | grep %s | cut -d, -f1)

# Remove user from group (will succeed silently if user is not in group)
/opt/keycloak/bin/kcadm.sh delete users/$USER_ID/groups/$GROUP_ID -r grid || true

echo "✓ Removed user %s from group %s"
`, username, groupName, username, groupName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dockerCmd := exec.CommandContext(ctx, "docker", "compose", "exec", "-T", "keycloak", "bash", "-c", removeFromGroupScript)
	output, err := dockerCmd.CombinedOutput()

	if err != nil {
		t.Logf("Warning: Failed to remove user from group: %v", err)
		t.Logf("Output: %s", string(output))
	} else {
		t.Logf("Removed user %s from group %s: %s", username, groupName, string(output))
	}
}
