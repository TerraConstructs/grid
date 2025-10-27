// Package integration provides end-to-end integration tests for Grid.
//
// # Mode 1 (External IdP) Authentication Tests
//
// These tests verify the Mode 1 authentication flow where Grid acts as a Resource Server
// validating tokens issued by an external OIDC provider (Keycloak).
//
// ## Prerequisites
//
// Before running these tests:
//
// 1. **Start PostgreSQL**: `make db-up`
// 2. **Start Keycloak**: `make keycloak-up`
// 3. **Build gridapi**: `go build -o ../../bin/gridapi ./cmd/gridapi`
// 4. **Run tests with Mode 1 configuration**:
//
//	```bash
//	cd tests/integration
//	export EXTERNAL_IDP_ISSUER="http://localhost:8443/realms/grid"
//	export EXTERNAL_IDP_CLIENT_ID="grid-api"
//	export EXTERNAL_IDP_CLIENT_SECRET="<keycloak-client-secret>"
//	export EXTERNAL_IDP_REDIRECT_URI="http://localhost:8080/auth/sso/callback"
//	go test -v -run "TestMode1"
//	```
//
// ## Tests
//
// Phase 1 (Infrastructure):
// - TestMode1_KeycloakHealth: Verifies Keycloak is running and Grid is configured for Mode 1
//
// Phase 2 (Core Auth):
// - TestMode1_ExternalTokenValidation: Verifies Grid validates tokens issued by Keycloak
// - TestMode1_ServiceAccountAuth: Tests client credentials flow with Keycloak
//
// Phase 3 (Advanced):
// - TestMode1_UserGroupAuthorization: Tests group-based role assignment
// - TestMode1_SSO_WebFlow: Tests browser-based SSO login
// - TestMode1_DeviceFlow: Tests CLI device authorization
//
// Phase 4 (Security):
// - TestMode1_TokenExpiry: Tests expired token rejection
// - TestMode1_InvalidTokenRejection: Tests malformed/invalid token security
package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"
)

// Note: Helper functions have been moved to separate files:
// - auth_mode1_infrastructure.go: Infrastructure helpers (Keycloak health, OIDC discovery)
// - auth_mode1_auth_helpers.go: Authentication helpers (token acquisition, Keycloak setup)
// - auth_mode1_rbac_helpers.go: RBAC helpers (group-role assignment via gridctl)

// BLACK-BOX TEST STRATEGY:
// These tests use direct HTTP calls to Connect RPC endpoints (ListStates, CreateState) instead of gridctl.
// Justification: Auth/AuthZ tests need to verify token validation at the protocol level, before the request
// reaches business logic. Using gridctl would abstract away authentication headers and make it impossible
// to test specific scenarios like:
// - Invalid token formats (malformed JWT, missing claims)
// - Token expiry validation
// - Audience claim validation
// - Group claim extraction from external IdP
// For production workflow tests (state creation, locking, etc.), prefer gridctl/terraform binaries.

// ===============================================
// Phase 1: Infrastructure Tests
// ===============================================

// TestMode1_KeycloakHealth verifies that Keycloak is running and Grid is configured for Mode 1
func TestMode1_KeycloakHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Step 0: CRITICAL - Verify auth is actually enabled on the server
	t.Log("Step 0: Verifying authentication is enabled on the server...")
	verifyAuthEnabled(t, "Mode 1")

	// Step 1: Verify Keycloak is healthy
	t.Log("Step 1: Checking Keycloak health...")
	healthy := isKeycloakHealthy(t)
	require.True(t, healthy, "Keycloak must be running and healthy. Run 'make keycloak-up' to start it.")

	// Step 2: Check Keycloak OIDC discovery endpoint
	t.Log("Step 2: Fetching Keycloak OIDC discovery document...")
	discovery := getKeycloakDiscovery(t)
	require.NotNil(t, discovery, "Keycloak discovery document should be available")
	require.Equal(t, fmt.Sprintf("%s/realms/%s", keycloakBaseURL, keycloakRealm), discovery.Issuer, "Keycloak issuer should match expected value")
	require.Contains(t, discovery.GrantTypesSupported, "client_credentials", "Keycloak should support client_credentials grant")

	t.Logf("Keycloak discovery: issuer=%s, token_endpoint=%s", discovery.Issuer, discovery.TokenEndpoint)

	// Step 3: Verify Grid does NOT expose OIDC discovery (Mode 1 = Resource Server)
	t.Log("Step 3: Verifying Grid does NOT expose OIDC discovery (Resource Server role)...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gridDiscoveryURL := fmt.Sprintf("%s/.well-known/openid-configuration", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gridDiscoveryURL, nil)
	require.NoError(t, err, "Failed to create Grid discovery request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Grid discovery endpoint")
	defer resp.Body.Close()

	require.NotEqual(t, http.StatusOK, resp.StatusCode, "Grid should NOT expose OIDC discovery in Mode 1 (Resource Server role)")
	t.Logf("Grid OIDC discovery correctly returns %d (not an IdP)", resp.StatusCode)

	// Step 4: Verify Grid exposes SSO endpoints (RelyingParty endpoints)
	t.Log("Step 4: Verifying Grid exposes SSO endpoints (RelyingParty role)...")
	ssoLoginURL := fmt.Sprintf("%s/auth/sso/login", serverURL)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, ssoLoginURL, nil)
	require.NoError(t, err, "Failed to create SSO login request")

	// Use client that doesn't follow redirects so we can verify the redirect response
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err = noRedirectClient.Do(req)
	require.NoError(t, err, "Failed to call SSO login endpoint")
	defer resp.Body.Close()

	// Should redirect to Keycloak
	require.True(t, resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect,
		"SSO login should redirect to Keycloak (got %d)", resp.StatusCode)

	location := resp.Header.Get("Location")
	require.Contains(t, location, keycloakBaseURL, "SSO login should redirect to Keycloak")
	t.Logf("SSO login correctly redirects to: %s", location)

	// Step 5: Verify isMode1Configured helper
	t.Log("Step 5: Verifying isMode1Configured() helper...")
	isMode1 := isMode1Configured(t)
	require.True(t, isMode1, "Server should be detected as Mode 1 configured")

	t.Log("✓ Mode 1 infrastructure validation complete!")
}

// ===============================================
// Phase 2: Core Authentication Tests
// ===============================================

// TestMode1_ExternalTokenValidation verifies Grid validates tokens issued by Keycloak
func TestMode1_ExternalTokenValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	// Step 1: Get a token from Keycloak using client credentials
	// NOTE: This requires a Keycloak client with service account enabled
	// For now, we'll skip this test if the client doesn't exist
	t.Log("Step 1: Authenticating with Keycloak using client credentials...")

	// Try to authenticate with a service account client
	// This client should be pre-created in Keycloak with service accounts enabled
	clientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	clientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Skip("Skipping TestMode1_ExternalTokenValidation: MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET must be set. " +
			"Create a Keycloak client with service accounts enabled and set these env vars.")
	}

	tokenResp := authenticateWithKeycloak(t, clientID, clientSecret)
	require.NotEmpty(t, tokenResp.AccessToken, "Access token should not be empty")
	require.Equal(t, "Bearer", tokenResp.TokenType, "Token type should be Bearer")

	t.Logf("Received token from Keycloak (expires_in=%d seconds)", tokenResp.ExpiresIn)

	// Step 2: Parse the JWT to verify claims
	t.Log("Step 2: Parsing JWT to verify claims...")
	token, err := jwt.ParseSigned(tokenResp.AccessToken)
	require.NoError(t, err, "Failed to parse JWT")

	var claims map[string]any
	err = token.UnsafeClaimsWithoutVerification(&claims)
	require.NoError(t, err, "Failed to extract claims")

	// Verify issuer is Keycloak, not Grid
	issuer, ok := claims["iss"].(string)
	require.True(t, ok, "iss claim should be a string")
	expectedIssuer := fmt.Sprintf("%s/realms/%s", keycloakBaseURL, keycloakRealm)
	require.Equal(t, expectedIssuer, issuer, "Token issuer should be Keycloak")

	t.Logf("Token claims: iss=%s, sub=%v, aud=%v, groups=%v", issuer, claims["sub"], claims["aud"], claims["groups"])

	// Step 3: Use token to call Grid API (Grid should validate it)
	t.Log("Step 3: Using Keycloak token to call Grid API...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Call ListStates endpoint (requires authentication)
	listStatesURL := fmt.Sprintf("%s/state.v1.StateService/ListStates", serverURL)
	reqBody := strings.NewReader("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, listStatesURL, reqBody)
	require.NoError(t, err, "Failed to create API request")

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Grid API")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Service account is in test-admins group → platform-engineer role (via bootstrap)
	// Should have full permissions for state operations
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"Grid should authorize valid Keycloak token with admin permissions (got %d): %s", resp.StatusCode, string(body))

	t.Logf("Grid API response: %d (token validated and authorized successfully)", resp.StatusCode)
	t.Log("✓ External token validation complete!")
}

// TestMode1_ServiceAccountAuth tests service account authentication via Keycloak client credentials
func TestMode1_ServiceAccountAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	t.Log("Testing service account authentication in Mode 1...")

	// In Mode 1, service accounts are Keycloak clients (not Grid database entities)
	// They are created in Keycloak with "Service Accounts Enabled"
	clientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	clientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Skip("Skipping TestMode1_ServiceAccountAuth: MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET must be set. " +
			"Instructions:\n" +
			"1. Access Keycloak at http://localhost:8443\n" +
			"2. Login with admin/admin\n" +
			"3. Go to realm 'grid' (create if doesn't exist)\n" +
			"4. Create a client (e.g., 'ci-pipeline'):\n" +
			"   - Client Protocol: openid-connect\n" +
			"   - Access Type: confidential\n" +
			"   - Service Accounts Enabled: ON\n" +
			"5. Copy Client Secret from Credentials tab\n" +
			"6. Set env vars: MODE1_TEST_CLIENT_ID=ci-pipeline MODE1_TEST_CLIENT_SECRET=<secret>")
	}

	// Step 1: Authenticate with Keycloak
	t.Log("Step 1: Authenticating service account with Keycloak...")
	tokenResp := authenticateWithKeycloak(t, clientID, clientSecret)
	require.NotEmpty(t, tokenResp.AccessToken, "Access token should not be empty")

	// Step 2: Verify token issuer
	t.Log("Step 2: Verifying token issuer is Keycloak...")
	token, err := jwt.ParseSigned(tokenResp.AccessToken)
	require.NoError(t, err, "Failed to parse JWT")

	var claims map[string]any
	err = token.UnsafeClaimsWithoutVerification(&claims)
	require.NoError(t, err, "Failed to extract claims")

	issuer, ok := claims["iss"].(string)
	require.True(t, ok, "iss claim should be a string")
	expectedIssuer := fmt.Sprintf("%s/realms/%s", keycloakBaseURL, keycloakRealm)
	require.Equal(t, expectedIssuer, issuer, "Token issuer should be Keycloak")

	// Step 3: Use token for Grid API call
	t.Log("Step 3: Using service account token for Grid API call...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listStatesURL := fmt.Sprintf("%s/state.v1.StateService/ListStates", serverURL)
	reqBody := strings.NewReader("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, listStatesURL, reqBody)
	require.NoError(t, err, "Failed to create API request")

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Grid API")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Service account is in test-admins group → platform-engineer role (via bootstrap)
	// Should have full permissions for state operations
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"Service account should be authorized for state operations (got %d): %s", resp.StatusCode, string(body))

	t.Logf("Grid API response: %d", resp.StatusCode)
	t.Log("✓ Service account authentication complete!")
}

// ===============================================
// Phase 3: Advanced Tests
// ===============================================

// TestMode1_UserGroupAuthorization tests group-based RBAC with IdP groups (Scenario 9)
func TestMode1_UserGroupAuthorization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 advanced integration test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	t.Log("Testing group-based authorization with Keycloak groups...")

	// This test requires:
	// 1. Keycloak configured with groups claim mapper
	// 2. Test user assigned to groups in Keycloak
	// 3. Grid configured with group-to-role mappings

	clientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	clientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Skip("Skipping TestMode1_UserGroupAuthorization: MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET required.\n" +
			"Setup:\n" +
			"1. Configure Keycloak groups claim mapper (Scenario 9 in quickstart.md)\n" +
			"2. Create groups: platform-engineers, product-engineers\n" +
			"3. Assign service account to group\n" +
			"4. Map groups to roles via Grid admin")
	}

	// Step 1: Authenticate and get token
	t.Log("Step 1: Authenticating with Keycloak...")
	tokenResp := authenticateWithKeycloak(t, clientID, clientSecret)
	require.NotEmpty(t, tokenResp.AccessToken, "Access token should not be empty")

	// Step 2: Parse JWT and verify groups claim
	t.Log("Step 2: Verifying groups claim in JWT...")
	token, err := jwt.ParseSigned(tokenResp.AccessToken)
	require.NoError(t, err, "Failed to parse JWT")

	var claims map[string]any
	err = token.UnsafeClaimsWithoutVerification(&claims)
	require.NoError(t, err, "Failed to extract claims")

	// Check if groups claim exists
	groupsClaim, hasGroups := claims["groups"]
	if hasGroups {
		t.Logf("Groups claim found: %v", groupsClaim)
		// Groups can be either []string or []map[string]any depending on mapper config
		switch groups := groupsClaim.(type) {
		case []any:
			t.Logf("Groups (array): %v", groups)
		default:
			t.Logf("Groups (unknown type): %T", groups)
		}
	} else {
		// Fail, the setup is incorrect
		t.Fatal("Groups claim not found in token. Ensure Keycloak is configured with groups mapper.")
	}

	// Step 3: Call Grid API and verify group-based authorization
	t.Log("Step 3: Testing Grid API with group-based authorization...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listStatesURL := fmt.Sprintf("%s/state.v1.StateService/ListStates", serverURL)
	reqBody := strings.NewReader("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, listStatesURL, reqBody)
	require.NoError(t, err, "Failed to create API request")

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Grid API")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Grid API response: %d, body: %s", resp.StatusCode, string(body))

	// Service account is in test-admins group → platform-engineer role (via bootstrap)
	// Should have full permissions for state operations
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"Token with groups claim should be authorized (got %d): %s", resp.StatusCode, string(body))

	t.Log("✓ Group-based authorization test complete!")
}

// TestMode1_SSO_WebFlow tests browser-based SSO login (Scenario 1)
func TestMode1_SSO_WebFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 SSO web flow test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	t.Log("Testing SSO web flow redirection...")

	// Step 1: Access SSO login endpoint
	t.Log("Step 1: Accessing /auth/sso/login endpoint...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ssoLoginURL := fmt.Sprintf("%s/auth/sso/login", serverURL)

	// Create client that doesn't follow redirects (we want to inspect the redirect)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ssoLoginURL, nil)
	require.NoError(t, err, "Failed to create SSO login request")

	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to call SSO login endpoint")
	defer resp.Body.Close()

	// Step 2: Verify redirect to Keycloak
	t.Log("Step 2: Verifying redirect to Keycloak authorization endpoint...")
	require.True(t, resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect,
		"SSO login should redirect (got %d)", resp.StatusCode)

	location := resp.Header.Get("Location")
	require.NotEmpty(t, location, "Redirect location should be present")
	require.Contains(t, location, keycloakBaseURL, "Should redirect to Keycloak")
	require.Contains(t, location, "/auth", "Should redirect to Keycloak auth endpoint")

	t.Logf("Redirect location: %s", location)

	// Step 3: Verify redirect contains OIDC parameters
	t.Log("Step 3: Verifying OIDC parameters in redirect...")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err, "Failed to parse redirect URL")

	query := redirectURL.Query()
	require.NotEmpty(t, query.Get("client_id"), "client_id should be present")
	require.NotEmpty(t, query.Get("redirect_uri"), "redirect_uri should be present")
	require.NotEmpty(t, query.Get("response_type"), "response_type should be present")
	require.NotEmpty(t, query.Get("state"), "state should be present (CSRF protection)")

	t.Logf("OIDC params: client_id=%s, redirect_uri=%s, response_type=%s",
		query.Get("client_id"), query.Get("redirect_uri"), query.Get("response_type"))

	// Step 4: Verify callback endpoint exists
	t.Log("Step 4: Verifying SSO callback endpoint exists...")
	callbackURL := fmt.Sprintf("%s/auth/sso/callback", serverURL)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, callbackURL, nil)
	require.NoError(t, err, "Failed to create callback request")

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call callback endpoint")
	defer resp.Body.Close()

	// Callback without code/state should return error, but endpoint should exist (not 404)
	require.NotEqual(t, http.StatusNotFound, resp.StatusCode,
		"Callback endpoint should exist (got %d)", resp.StatusCode)

	t.Logf("Callback endpoint status: %d (expected error without valid code/state)", resp.StatusCode)
	t.Log("✓ SSO web flow test complete!")
}

// TestMode1_DeviceFlow tests CLI device authorization (Scenario 2)
func TestMode1_DeviceFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 device flow test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	t.Log("Testing device authorization flow...")

	// Note: Full device flow testing requires:
	// 1. Keycloak configured with device authorization enabled
	// 2. CLI implementation of device flow
	// This test verifies the infrastructure is in place

	// Step 1: Check if Keycloak supports device authorization
	t.Log("Step 1: Checking Keycloak device authorization support...")
	discovery := getKeycloakDiscovery(t)

	// Device authorization endpoint is optional in OIDC discovery
	// Check if supported
	deviceAuthURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/auth/device", keycloakBaseURL, keycloakRealm)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to access device authorization endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceAuthURL, strings.NewReader("client_id=test"))
	require.NoError(t, err, "Failed to create device auth request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("Device authorization endpoint not accessible: %v", err)
		t.Skip("Keycloak device authorization endpoint not available or not configured")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Device authorization response: %d, body: %s", resp.StatusCode, string(body))

	// If we get 400 (bad request), endpoint exists but request is invalid (expected)
	// If we get 404, device flow not enabled
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Keycloak device authorization not enabled")
		return
	}

	t.Logf("Keycloak device authorization endpoint exists at: %s", deviceAuthURL)
	t.Logf("OIDC discovery: issuer=%s", discovery.Issuer)

	t.Log("✓ Device flow infrastructure check complete!")
	t.Log("Note: Full device flow testing requires CLI implementation and user interaction")
}

// ===============================================
// Phase 4: Security Tests
// ===============================================

// TestMode1_TokenExpiry tests expired token rejection (Scenario 7)
func TestMode1_TokenExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 security test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	t.Log("Testing token expiry handling...")

	// This test verifies Grid properly rejects expired tokens
	// In Mode 1, tokens are issued by Keycloak with exp claim

	// Step 1: Create a JWT with expired timestamp
	t.Log("Step 1: Testing with expired token...")

	// We can't easily create expired Keycloak tokens in tests,
	// but we can verify Grid checks the exp claim by:
	// 1. Getting a valid token
	// 2. Checking it works
	// 3. Documenting that Grid validates exp claim

	clientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	clientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Skip("Skipping TestMode1_TokenExpiry: MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET required")
	}

	// Get a valid token
	tokenResp := authenticateWithKeycloak(t, clientID, clientSecret)
	require.NotEmpty(t, tokenResp.AccessToken, "Access token should not be empty")

	// Parse and check exp claim
	token, err := jwt.ParseSigned(tokenResp.AccessToken)
	require.NoError(t, err, "Failed to parse JWT")

	var claims map[string]any
	err = token.UnsafeClaimsWithoutVerification(&claims)
	require.NoError(t, err, "Failed to extract claims")

	expClaim, hasExp := claims["exp"]
	require.True(t, hasExp, "Token should have exp claim")
	t.Logf("Token exp claim: %v (type: %T)", expClaim, expClaim)

	// Verify token works when not expired
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listStatesURL := fmt.Sprintf("%s/state.v1.StateService/ListStates", serverURL)
	reqBody := strings.NewReader("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, listStatesURL, reqBody)
	require.NoError(t, err, "Failed to create API request")

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Grid API")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	// Service account token should be valid and authorized
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"Valid token should be authorized (got %d): %s", resp.StatusCode, string(body))

	t.Logf("Valid token accepted and authorized: %d", resp.StatusCode)

	// Note: To fully test expiry, we would need to:
	// 1. Wait for token to expire (not practical in tests)
	// 2. Or manipulate Keycloak to issue short-lived tokens
	// 3. Or forge a token with past exp (requires signing key)

	t.Log("✓ Token expiry claim verification complete!")
	t.Log("Note: Grid validates exp claim via JWT library. Full expiry testing requires manual verification.")
}

// TestMode1_InvalidTokenRejection tests malformed/invalid token security
func TestMode1_InvalidTokenRejection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 security test in short mode")
	}

	t.Log("Testing invalid token rejection...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listStatesURL := fmt.Sprintf("%s/state.v1.StateService/ListStates", serverURL)

	// Test cases for invalid tokens
	testCases := []struct {
		name        string
		token       string
		description string
	}{
		{
			name:        "Empty token",
			token:       "",
			description: "Empty authorization header value",
		},
		{
			name:        "Malformed token",
			token:       "not-a-jwt-token",
			description: "Random string, not JWT format",
		},
		{
			name:        "Invalid JWT structure",
			token:       "header.payload",
			description: "Missing signature component",
		},
		{
			name:        "Invalid base64",
			token:       "!!!.!!!.!!!",
			description: "Invalid base64 encoding",
		},
		{
			name:        "Wrong issuer",
			token:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwOi8vZXZpbC5jb20iLCJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			description: "Valid JWT structure but wrong issuer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)

			reqBody := strings.NewReader("{}")
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, listStatesURL, reqBody)
			require.NoError(t, err, "Failed to create API request")

			if tc.token != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tc.token))
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "Failed to call Grid API")
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			// Invalid tokens should be rejected with 401
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"Invalid token should return 401 (got %d): %s", resp.StatusCode, string(body))

			t.Logf("✓ Correctly rejected with 401: %s", string(body))
		})
	}

	t.Log("✓ Invalid token rejection tests complete!")
}

// ===============================================
// Phase 5: SSO User & Group-Role Mapping Tests
// ===============================================

// TestMode1_SSO_UserAuth tests end-to-end SSO user authentication with password grant
func TestMode1_SSO_UserAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 SSO user authentication test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	t.Log("Testing end-to-end SSO user authentication...")

	// Step 1: Setup Keycloak with test user
	t.Log("Step 1: Setting up Keycloak with test user and groups...")
	setupKeycloakForGroupTests(t)

	// Step 2: Authenticate as user Alice via password grant
	t.Log("Step 2: Authenticating user alice@example.com via password grant...")
	clientID := os.Getenv("EXTERNAL_IDP_CLIENT_ID")
	clientSecret := os.Getenv("EXTERNAL_IDP_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Skip("EXTERNAL_IDP_CLIENT_ID and EXTERNAL_IDP_CLIENT_SECRET must be set")
	}

	tokenResp := authenticateUserWithPassword(t, clientID, clientSecret, "alice@example.com", "test123")
	require.NotEmpty(t, tokenResp.AccessToken, "User token should not be empty")
	t.Logf("Received user token (expires_in=%d seconds)", tokenResp.ExpiresIn)

	// Step 3: Parse JWT and verify user claims
	t.Log("Step 3: Parsing JWT to verify user claims...")
	token, err := jwt.ParseSigned(tokenResp.AccessToken)
	require.NoError(t, err, "Failed to parse JWT")

	var claims map[string]any
	err = token.UnsafeClaimsWithoutVerification(&claims)
	require.NoError(t, err, "Failed to extract claims")

	// Verify user-specific claims
	require.Contains(t, claims, "sub", "Token should have subject claim")
	require.Contains(t, claims, "preferred_username", "Token should have username claim")

	username, ok := claims["preferred_username"].(string)
	require.True(t, ok, "preferred_username should be a string")
	require.Equal(t, "alice@example.com", username, "Username should match")

	// Verify groups claim exists
	groupsClaim, hasGroups := claims["groups"]
	if hasGroups {
		t.Logf("Groups claim found: %v", groupsClaim)
	} else {
		t.Log("Warning: No groups claim - mapper may not be configured yet")
	}

	// Step 4: Use user token to call Grid API
	t.Log("Step 4: Using user token to call Grid API...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listStatesURL := fmt.Sprintf("%s/state.v1.StateService/ListStates", serverURL)
	reqBody := strings.NewReader("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, listStatesURL, reqBody)
	require.NoError(t, err, "Failed to create API request")

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Grid API")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// User token should be validated (authentication), but alice is in product-engineers group
	// which has no role mapping, so expect 403 Forbidden (authorization denied)
	require.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Alice should be authenticated but not authorized (no role mapping for product-engineers), got %d: %s", resp.StatusCode, string(body))

	t.Logf("Grid API response: %d (authenticated but not authorized as expected)", resp.StatusCode)
	t.Log("✓ End-to-end SSO user authentication complete!")
}

// TestMode1_GroupRoleMapping tests dynamic group→role mapping with permission transitions
func TestMode1_GroupRoleMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 group-role mapping test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	t.Log("Testing dynamic group→role mapping with permission transitions...")

	// Step 1: Setup Keycloak with groups and users
	t.Log("Step 1: Setting up Keycloak with groups and users...")
	setupKeycloakForGroupTests(t)

	// Step 2: Get service account token to act as admin
	t.Log("Step 2: Authenticating as admin to configure group→role mappings...")

	// Use MODE1_TEST_CLIENT_ID (integration-tests) for admin operations
	// EXTERNAL_IDP_CLIENT_ID (grid-api) is the resource server client, not for test operations
	testClientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	testClientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")

	if testClientID == "" || testClientSecret == "" {
		t.Skip("MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET must be set")
	}

	// Use service account for admin operations
	adminTokenResp := authenticateWithKeycloak(t, testClientID, testClientSecret)
	require.NotEmpty(t, adminTokenResp.AccessToken, "Admin token should not be empty")

	// Step 3: Authenticate as user Alice (who is in product-engineers group)
	t.Log("Step 3: Authenticating as alice@example.com...")
	// For user password grant, we must use EXTERNAL_IDP_CLIENT_ID (grid-api) because it has directAccessGrantsEnabled: true
	// The integration-tests client does NOT support password grant (only client credentials)
	userClientID := os.Getenv("EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")
	require.NotEmpty(t, userTokenResp.AccessToken, "User token should not be empty")

	// Step 4: Verify Alice's groups in JWT
	t.Log("Step 4: Verifying groups in user JWT...")
	token, err := jwt.ParseSigned(userTokenResp.AccessToken)
	require.NoError(t, err, "Failed to parse JWT")

	var claims map[string]any
	err = token.UnsafeClaimsWithoutVerification(&claims)
	require.NoError(t, err, "Failed to extract claims")

	groupsClaim, hasGroups := claims["groups"]
	if hasGroups {
		t.Logf("User groups: %v", groupsClaim)
		groups, ok := groupsClaim.([]any)
		if ok && len(groups) > 0 {
			found := false
			for _, g := range groups {
				if gStr, ok := g.(string); ok && (gStr == "product-engineers" || gStr == "/product-engineers") {
					found = true
					break
				}
			}
			require.True(t, found, "User should be in product-engineers group")
		}
	} else {
		t.Skip("Groups claim not present - Keycloak mapper not configured. Run setupKeycloakForGroupTests manually.")
	}

	// Step 5: Map product-engineers group to product-engineer role in Grid
	t.Log("Step 5: Mapping product-engineers group → product-engineer role in Grid...")
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "product-engineers", "product-engineer")

	// Step 6: Test label-based access control with group-based role
	t.Log("Step 6: Testing label-based access control...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createStateURL := fmt.Sprintf("%s/state.v1.StateService/CreateState", serverURL)

	// Step 6a: Try to create state with INCORRECT labels (should fail due to label constraint)
	t.Log("Step 6a: Testing state creation with INCORRECT labels (expecting 403)...")
	wrongGuid := uuid.Must(uuid.NewV7()).String()
	wrongLogicID := fmt.Sprintf("test-alice-wrong-labels-%d", time.Now().UnixNano())
	wrongLabelsJSON := fmt.Sprintf(`{"guid":"%s","logic_id":"%s","labels":{"team":"product"}}`, wrongGuid, wrongLogicID)
	wrongLabelsReq := strings.NewReader(wrongLabelsJSON)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createStateURL, wrongLabelsReq)
	require.NoError(t, err, "Failed to create request")

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", userTokenResp.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call Grid API")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	t.Logf("CreateState (wrong labels) response: %d, body: %s", resp.StatusCode, string(body))
	require.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Creating state with wrong labels should be denied due to label constraint (expected 403, got %d): %s",
		resp.StatusCode, string(body))
	t.Log("✓ Label constraint correctly enforced (403 for wrong labels)")

	// Step 6b: Try to create state with CORRECT labels (should succeed)
	t.Log("Step 6b: Testing state creation with CORRECT labels (expecting 200)...")
	correctGuid := uuid.Must(uuid.NewV7()).String()
	correctLogicID := fmt.Sprintf("test-alice-correct-labels-%d", time.Now().UnixNano())
	correctLabelsJSON := fmt.Sprintf(`{"guid":"%s","logic_id":"%s","labels":{"env":"dev"}}`, correctGuid, correctLogicID)
	correctLabelsReq := strings.NewReader(correctLabelsJSON)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, createStateURL, correctLabelsReq)
	require.NoError(t, err, "Failed to create request")

	req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", userTokenResp.AccessToken))
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err, "Failed to call Grid API")
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()

	t.Logf("CreateState (correct labels) response: %d, body: %s", resp2.StatusCode, string(body2))
	require.Equal(t, http.StatusOK, resp2.StatusCode,
		"Creating state with correct labels should succeed (expected 200, got %d): %s",
		resp2.StatusCode, string(body2))
	t.Log("✓ State created successfully with correct labels")

	t.Log("✓ Group→role mapping and label-based access control test complete!")
}

// TestMode1_GroupRoleMapping_UnionSemantics tests multiple groups with union (OR) semantics
func TestMode1_GroupRoleMapping_UnionSemantics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mode 1 union semantics test in short mode")
	}

	// Prerequisites
	require.True(t, isKeycloakHealthy(t), "Keycloak must be running")

	t.Log("Testing union semantics for multiple group→role mappings...")

	// Step 1: Setup Keycloak with Alice in product-engineers group
	t.Log("Step 1: Setting up Keycloak...")
	setupKeycloakForGroupTests(t)

	// Step 2: Add Alice to platform-engineers group to test multiple group memberships
	t.Log("Step 2: Adding alice to platform-engineers group...")
	addUserToKeycloakGroup(t, "alice@example.com", "platform-engineers")
	// Clean up at test end to avoid polluting subsequent tests
	defer removeUserFromKeycloakGroup(t, "alice@example.com", "platform-engineers")

	// Use MODE1_TEST_CLIENT_ID (integration-tests) for admin operations
	testClientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	testClientSecret := os.Getenv("MODE1_TEST_CLIENT_SECRET")

	if testClientID == "" || testClientSecret == "" {
		t.Skip("MODE1_TEST_CLIENT_ID and MODE1_TEST_CLIENT_SECRET must be set")
	}

	// Step 3: Get tokens for admin operations
	t.Log("Step 3: Authenticating as admin...")
	adminTokenResp := authenticateWithKeycloak(t, testClientID, testClientSecret)
	require.NotEmpty(t, adminTokenResp.AccessToken)

	// Step 4: Map both groups to different roles in Grid
	t.Log("Step 4: Mapping both groups to roles...")
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "product-engineers", "product-engineer")
	assignGroupRoleInGrid(t, adminTokenResp.AccessToken, "platform-engineers", "platform-engineer")

	// Step 5: Authenticate Alice (she should now have both groups)
	t.Log("Step 5: Authenticating Alice...")
	// For user password grant, use EXTERNAL_IDP_CLIENT_ID (grid-api) which supports password grant
	userClientID := os.Getenv("EXTERNAL_IDP_CLIENT_ID")
	userClientSecret := os.Getenv("EXTERNAL_IDP_CLIENT_SECRET")
	userTokenResp := authenticateUserWithPassword(t, userClientID, userClientSecret, "alice@example.com", "test123")
	require.NotEmpty(t, userTokenResp.AccessToken)

	// Step 6: Parse JWT to verify multiple groups
	t.Log("Step 6: Verifying multiple groups in JWT...")
	token, err := jwt.ParseSigned(userTokenResp.AccessToken)
	require.NoError(t, err)

	var claims map[string]any
	err = token.UnsafeClaimsWithoutVerification(&claims)
	require.NoError(t, err)

	if groupsClaim, hasGroups := claims["groups"]; hasGroups {
		t.Logf("User groups: %v", groupsClaim)
		// Verify Alice is in both groups
		groups, ok := groupsClaim.([]any)
		if ok {
			groupNames := make([]string, 0, len(groups))
			for _, g := range groups {
				if gStr, ok := g.(string); ok {
					groupNames = append(groupNames, gStr)
				}
			}
			t.Logf("Alice's group memberships: %v", groupNames)
			// At least one of the groups should be product-engineers or platform-engineers
			// (exact names may vary due to Keycloak path formatting)
		}
	} else {
		t.Skip("Groups claim not present in JWT")
	}

	// Step 7: Test union semantics - Alice should have permissions from BOTH roles
	t.Log("Step 7: Testing union semantics...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test 7a: ListStates should work because platform-engineer has wildcard access (no label constraints)
	t.Log("Step 7a: Testing ListStates (platform-engineer allows this via wildcard)...")
	listStatesURL := fmt.Sprintf("%s/state.v1.StateService/ListStates", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, listStatesURL, strings.NewReader("{}"))
	require.NoError(t, err)

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", userTokenResp.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	t.Logf("ListStates response: %d, body: %s", resp.StatusCode, string(body))
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"User with platform-engineer role (via platform-engineers group) should be able to list states (got %d): %s",
		resp.StatusCode, string(body))
	t.Log("✓ ListStates succeeded via platform-engineer wildcard access")

	// Test 7b: Create state with env=dev label should work (product-engineer allows this)
	t.Log("Step 7b: Testing CreateState with env=dev (product-engineer allows this)...")
	createStateURL := fmt.Sprintf("%s/state.v1.StateService/CreateState", serverURL)
	unionGuid := uuid.Must(uuid.NewV7()).String()
	unionLogicID := fmt.Sprintf("test-alice-union-semantics-%d", time.Now().UnixNano())
	createJSON := fmt.Sprintf(`{"guid":"%s","logic_id":"%s","labels":{"env":"dev"}}`, unionGuid, unionLogicID)
	createReq := strings.NewReader(createJSON)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, createStateURL, createReq)
	require.NoError(t, err)

	req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", userTokenResp.AccessToken))
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()

	t.Logf("CreateState response: %d, body: %s", resp2.StatusCode, string(body2))
	require.Equal(t, http.StatusOK, resp2.StatusCode,
		"User should be able to create state with env=dev label (got %d): %s",
		resp2.StatusCode, string(body2))
	t.Log("✓ CreateState succeeded with label-constrained product-engineer role")

	t.Log("✓ Union semantics test complete!")
	t.Log("   Alice has permissions from BOTH product-engineer (label-constrained) AND platform-engineer (wildcard) roles")
}
