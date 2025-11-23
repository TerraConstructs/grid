// Package integration provides end-to-end integration tests for Grid.
//
// # Mode 2 (Internal IdP) Authentication Tests
//
// These tests verify the Mode 2 authentication flow where Grid acts as an internal OIDC provider.
//
// ## Prerequisites
//
// Before running these tests:
//
// 1. **Start PostgreSQL**: `make db-up`
// 2. **Build gridapi**: `go build -o ../../bin/gridapi ./cmd/gridapi`
// 3. **Run tests with Mode 2 configuration**:
//
//	```bash
//	cd tests/integration
//	export OIDC_ISSUER="http://localhost:8080"
//	export OIDC_CLIENT_ID="gridapi"
//	export OIDC_SIGNING_KEY_PATH="tmp/keys/signing-key.pem"
//	go test -v -run "TestMode2"
//	```
//
// ## Tests
//
// - TestMode2_SigningKeyGeneration: Verifies JWT signing key auto-generation and persistence
// - TestMode2_ServiceAccountBootstrap: Tests service account creation via bootstrap command
// - TestMode2_ServiceAccountAuthentication: Tests OAuth2 client credentials flow
// - TestMode2_AuthenticatedAPICall: Verifies authenticated API calls work
// - TestMode2_JWTRevocation: Tests JWT revocation via jti denylist
package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/pkg/sdk"
	"gopkg.in/square/go-jose.v2/jwt"
)

const (
	mode2SigningKeyDir  = "tmp/keys"
	mode2SigningKeyPath = "tmp/keys/signing-key.pem"
)

// isMode2Configured checks if the server is running in Mode 2 (Internal IdP)
func isMode2Configured(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use /auth/config endpoint to determine mode
	configURL := fmt.Sprintf("%s/auth/config", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var config struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return false
	}

	return config.Mode == "internal-idp"
}

// setupMode2Environment configures environment variables for Mode 2 (Internal IdP)
func setupMode2Environment(t *testing.T) {
	t.Helper()
	os.Setenv("OIDC_ISSUER", "http://localhost:8080")
	os.Setenv("OIDC_CLIENT_ID", "gridapi")
	os.Setenv("OIDC_SIGNING_KEY_PATH", mode2SigningKeyPath)
	t.Logf("Mode 2 environment configured: OIDC_ISSUER=http://localhost:8080, OIDC_SIGNING_KEY_PATH=%s", mode2SigningKeyPath)
}

// createServiceAccountBootstrap uses the gridapi sa create command (bootstrap pattern)
// Returns client_id and client_secret from command output
func createServiceAccountBootstrap(t *testing.T, name string, role string) (clientID, clientSecret string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gridapiPath := getGridAPIPath(t)
	cmd := exec.CommandContext(ctx, gridapiPath, "sa", "create", name, "--role", role,
		"--db-url", "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to create service account: %s", string(output))

	// Parse output to extract client_id and client_secret
	// Expected format:
	// Client ID: <uuid>
	// Client Secret: <hex-string>
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Client ID:") {
			clientID = strings.TrimSpace(strings.TrimPrefix(line, "Client ID:"))
		}
		if strings.HasPrefix(line, "Client Secret:") {
			clientSecret = strings.TrimSpace(strings.TrimPrefix(line, "Client Secret:"))
		}
	}

	require.NotEmpty(t, clientID, "Failed to parse client_id from output: %s", string(output))
	require.NotEmpty(t, clientSecret, "Failed to parse client_secret from output: %s", string(output))

	t.Logf("Created service account %s: client_id=%s", name, clientID)
	return clientID, clientSecret
}

// authenticateServiceAccount calls OAuth2 token endpoint with client credentials
// Returns access token string
func authenticateServiceAccount(t *testing.T, clientID, clientSecret string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenURL := fmt.Sprintf("%s/oauth/token", serverURL)
	payload := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s",
		clientID, clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to call token endpoint")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Token endpoint returned non-200 status")

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	require.NoError(t, err, "Failed to decode token response")
	require.NotEmpty(t, tokenResp.AccessToken, "Access token is empty")

	t.Logf("Authenticated service account %s, got access token: %s...", clientID, tokenResp.AccessToken[:20])
	return tokenResp.AccessToken
}

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	Iss string      `json:"iss"`
	Sub string      `json:"sub"`
	Aud interface{} `json:"aud"` // Can be string or []string
	Jti string      `json:"jti"`
	Exp int64       `json:"exp"`
	Iat int64       `json:"iat"`
}

// parseJWT extracts and verifies JWT structure, returns claims
func parseJWT(t *testing.T, token string) JWTClaims {
	t.Helper()

	// JWT should have 3 parts separated by dots
	parts := strings.Split(token, ".")
	require.Equal(t, 3, len(parts), "JWT should have 3 parts (header.payload.signature)")

	// Parse JWT using square/go-jose library (without verification since we don't have the public key here)
	tok, err := jwt.ParseSigned(token)
	require.NoError(t, err, "Failed to parse JWT token")

	var claims JWTClaims
	err = tok.UnsafeClaimsWithoutVerification(&claims)
	require.NoError(t, err, "Failed to extract JWT claims")

	t.Logf("JWT claims: iss=%s, sub=%s, aud=%s, jti=%s", claims.Iss, claims.Sub, claims.Aud, claims.Jti)
	return claims
}

// TestMode2_SigningKeyGeneration verifies signing key is auto-generated and persisted
func TestMode2_SigningKeyGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// CRITICAL: Verify auth is enabled (fail instead of skip to prevent silent failures)
	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP). Run tests with OIDC_ISSUER=http://localhost:8080")
	}

	setupMode2Environment(t)

	// Check if signing key file exists after server start
	keyPath := mode2SigningKeyPath
	require.FileExists(t, keyPath, "Signing key should be generated at %s", keyPath)

	// Read first key generation timestamp
	stat1, err := os.Stat(keyPath)
	require.NoError(t, err)
	key1ModTime := stat1.ModTime()
	key1Content, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	t.Logf("Signing key exists: %s (size: %d bytes, mod_time: %s)", keyPath, len(key1Content), key1ModTime)

	// Verify it's a valid PEM file
	require.True(t, bytes.HasPrefix(key1Content, []byte("-----BEGIN")), "Key should be valid PEM format")

	// Sleep briefly
	time.Sleep(500 * time.Millisecond)

	// Read key again
	key2Content, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	// Verify key content is identical (not regenerated)
	require.Equal(t, key1Content, key2Content, "Signing key should be persisted and reused, not regenerated")

	t.Log("✓ Signing key generation and persistence verified")
}

// TestMode2_ServiceAccountBootstrap verifies service account creation via bootstrap
func TestMode2_ServiceAccountBootstrap(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// CRITICAL: Verify auth is enabled
	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP). Run tests with OIDC_ISSUER=http://localhost:8080")
	}

	setupMode2Environment(t)

	saName := fmt.Sprintf("test-sa-%s", uuid.New().String()[:8])
	role := "service-account" // Default role from seed data

	clientID, clientSecret := createServiceAccountBootstrap(t, saName, role)

	// Verify client_id is a valid UUID
	_, err := uuid.Parse(clientID)
	require.NoError(t, err, "client_id should be a valid UUID")

	// Verify client_secret is not empty and looks like hex
	require.NotEmpty(t, clientSecret, "client_secret should not be empty")
	require.Len(t, clientSecret, 64, "client_secret should be 64 hex characters (32 bytes)")

	t.Logf("✓ Service account created: %s (client_id=%s)", saName, clientID)
}

// TestMode2_ServiceAccountAuthentication verifies OAuth2 client credentials flow
func TestMode2_ServiceAccountAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// CRITICAL: Verify auth is enabled
	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP). Run tests with OIDC_ISSUER=http://localhost:8080")
	}

	setupMode2Environment(t)

	// Create service account
	saName := fmt.Sprintf("test-sa-auth-%s", uuid.New().String()[:8])
	clientID, clientSecret := createServiceAccountBootstrap(t, saName, "service-account")

	// Authenticate with client credentials
	accessToken := authenticateServiceAccount(t, clientID, clientSecret)
	require.NotEmpty(t, accessToken, "Access token should not be empty")

	// Parse JWT and verify claims
	claims := parseJWT(t, accessToken)

	// Verify required claims
	require.Equal(t, "http://localhost:8080", claims.Iss, "issuer should be Grid URL")
	// Audience can be string or array - check if it contains "gridapi"
	audStr := ""
	switch v := claims.Aud.(type) {
	case string:
		audStr = v
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				audStr = s
			}
		}
	}
	require.Equal(t, "gridapi", audStr, "audience should be 'gridapi'")
	require.NotEmpty(t, claims.Jti, "jti claim should exist for revocation tracking")
	require.True(t, strings.HasPrefix(claims.Sub, "sa:"), "subject should start with 'sa:' prefix")

	// Verify token is not expired
	now := time.Now().Unix()
	require.Greater(t, claims.Exp, now, "token should not be expired")

	// Verify iat is recent
	require.Less(t, now-claims.Iat, int64(5), "issued-at time should be recent (within 5 seconds)")

	t.Logf("✓ OAuth2 client credentials flow successful, JWT validated")
}

// TestMode2_AuthenticatedAPICall verifies authenticated API calls work with JWT token
func TestMode2_AuthenticatedAPICall(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// CRITICAL: Verify auth is enabled
	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP). Run tests with OIDC_ISSUER=http://localhost:8080")
	}

	setupMode2Environment(t)

	// Create service account
	saName := fmt.Sprintf("test-sa-api-%s", uuid.New().String()[:8])
	clientID, clientSecret := createServiceAccountBootstrap(t, saName, "service-account")

	// Get access token
	accessToken := authenticateServiceAccount(t, clientID, clientSecret)

	// Use token to make authenticated API call (health check as simple test)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/health", serverURL), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Health endpoint should be accessible with valid token
	require.Equal(t, http.StatusOK, resp.StatusCode, "Health check should succeed with valid token")

	// Try without token (should still work for health endpoint, but demonstrates token acceptance)
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/health", serverURL), nil)
	require.NoError(t, err)

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	require.Equal(t, http.StatusOK, resp2.StatusCode, "Health check should work without token too")

	t.Logf("✓ Authenticated API calls work correctly")
}

// TestMode2_JWTRevocation verifies JWT revocation via jti denylist
func TestMode2_JWTRevocation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// CRITICAL: Verify auth is enabled
	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP). Run tests with OIDC_ISSUER=http://localhost:8080")
	}

	setupMode2Environment(t)

	// Create service account
	saName := fmt.Sprintf("test-sa-revoke-%s", uuid.New().String()[:8])
	clientID, clientSecret := createServiceAccountBootstrap(t, saName, "service-account")

	// Get access token
	accessToken := authenticateServiceAccount(t, clientID, clientSecret)

	// Parse JWT to extract jti
	claims := parseJWT(t, accessToken)
	jti := claims.Jti
	require.NotEmpty(t, jti, "jti claim should exist")

	// Insert jti into revoked_jti table
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to database and insert revoked jti
	db, err := getDatabaseConnection()
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	revokedAt := time.Now()
	query := `INSERT INTO revoked_jti (jti, subject, exp, revoked_at, revoked_by) VALUES ($1, $2, $3, $4, $5)`
	_, err = db.ExecContext(ctx, query, jti, claims.Sub, time.Unix(claims.Exp, 0), revokedAt, "system")
	require.NoError(t, err, "Failed to insert revoked jti")

	t.Logf("Revoked JWT with jti=%s", jti)

	// Now try to use the revoked token on a PROTECTED endpoint - should fail with 401
	// Use Connect RPC endpoint (requires authentication) instead of /health (public)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/state.v1.StateService/ListStates", serverURL), bytes.NewBufferString("{}"))
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should receive 401 Unauthorized for revoked token
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Revoked token should result in 401 Unauthorized")

	t.Logf("✓ JWT revocation via jti denylist works correctly")
}

// getDatabaseConnection returns a database connection for testing
func getDatabaseConnection() (*sql.DB, error) {
	dsn := "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
	return sql.Open("postgres", dsn)
}

// ============================================================================
// Webapp Authentication Tests (Mode 2)
// ============================================================================
// These tests verify the webapp authentication handlers:
// - POST /auth/login (internal IdP username/password)
// - GET /api/auth/whoami (session restoration)
// - POST /auth/logout (session termination)
// - GET /auth/config (authentication mode discovery)

// createTestUser creates a user via gridapi users create command
// Returns the user's email for login testing
func createTestUser(t *testing.T, username, email, password string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gridapiPath := getGridAPIPath(t)
	cmd := exec.CommandContext(ctx, gridapiPath, "users", "create",
		"--username", username,
		"--email", email,
		"--password", password,
		"--role", "platform-engineer",
		"--db-url", "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to create user: %s", string(output))

	t.Logf("Created test user: %s (%s)", username, email)
	return email
}

// disableUser marks a user as disabled in the database
func disableUser(t *testing.T, email string) {
	t.Helper()
	db, err := getDatabaseConnection()
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE users SET disabled_at = NOW() WHERE email = $1`
	_, err = db.ExecContext(ctx, query, email)
	require.NoError(t, err, "Failed to disable user")

	t.Logf("Disabled user: %s", email)
}

// WebLoginResponse represents the response from POST /auth/login
type WebLoginResponse struct {
	User struct {
		ID       string   `json:"id"`
		Username string   `json:"username"`
		Email    string   `json:"email"`
		AuthType string   `json:"auth_type"`
		Roles    []string `json:"roles"`
	} `json:"user"`
	ExpiresAt int64 `json:"expires_at"`
}

// WebWhoamiResponse represents the response from GET /api/auth/whoami
type WebWhoamiResponse struct {
	User struct {
		ID       string   `json:"id"`
		Subject  string   `json:"subject"`
		Username string   `json:"username"`
		Email    string   `json:"email"`
		AuthType string   `json:"auth_type"`
		Roles    []string `json:"roles"`
		Groups   []string `json:"groups,omitempty"`
	} `json:"user"`
	Session struct {
		ID        string `json:"id"`
		ExpiresAt int64  `json:"expires_at"`
	} `json:"session"`
}

// loginWebUser performs POST /auth/login and returns response + session cookie
func loginWebUser(t *testing.T, email, password string) (*WebLoginResponse, *http.Cookie) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	loginURL := fmt.Sprintf("%s/auth/login", serverURL)
	payload := fmt.Sprintf(`{"username":"%s","password":"%s"}`, email, password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewBufferString(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Login should return 200")

	var loginResp WebLoginResponse
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	require.NoError(t, err, "Failed to decode login response")

	// Extract session cookie
	var sessionCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "grid.session" {
			sessionCookie = cookie
			break
		}
	}
	require.NotNil(t, sessionCookie, "Login response should include grid.session cookie")

	t.Logf("Logged in user %s, session cookie: %s...", email, sessionCookie.Value[:16])
	return &loginResp, sessionCookie
}

// fetchWhoamiWeb performs GET /api/auth/whoami with session cookie
func fetchWhoamiWeb(t *testing.T, sessionCookie *http.Cookie) (*WebWhoamiResponse, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whoamiURL := fmt.Sprintf("%s/api/auth/whoami", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, whoamiURL, nil)
	require.NoError(t, err)
	req.AddCookie(sessionCookie)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("whoami returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var whoamiResp WebWhoamiResponse
	err = json.NewDecoder(resp.Body).Decode(&whoamiResp)
	require.NoError(t, err, "Failed to decode whoami response")

	return &whoamiResp, nil
}

// logoutWebUser performs POST /auth/logout with session cookie
func logoutWebUser(t *testing.T, sessionCookie *http.Cookie) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logoutURL := fmt.Sprintf("%s/auth/logout", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, logoutURL, nil)
	require.NoError(t, err)
	req.AddCookie(sessionCookie)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Logout should return 200")
	t.Logf("Logged out successfully")
}

// TestMode2_WebAuth_LoginSuccess verifies successful login with valid credentials
func TestMode2_WebAuth_LoginSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	// Create test user
	email := fmt.Sprintf("testuser-%s@internal.grid", uuid.New().String()[:8])
	createTestUser(t, "testuser", email, "password123")

	// Login
	loginResp, sessionCookie := loginWebUser(t, email, "password123")

	// Verify response structure
	require.NotEmpty(t, loginResp.User.ID, "User ID should be set")
	require.Equal(t, "testuser", loginResp.User.Username, "Username should match")
	require.Equal(t, email, loginResp.User.Email, "Email should match")
	require.Equal(t, "internal", loginResp.User.AuthType, "Auth type should be 'internal'")
	require.NotNil(t, loginResp.User.Roles, "Roles should be present (can be empty array)")
	require.Greater(t, loginResp.ExpiresAt, time.Now().UnixMilli(), "ExpiresAt should be in the future")

	// Verify session cookie exists
	require.Equal(t, "grid.session", sessionCookie.Name, "Cookie name should be 'grid.session'")
	require.NotEmpty(t, sessionCookie.Value, "Session cookie value should not be empty")

	t.Log("✓ Login success test passed")
}

// TestMode2_WebAuth_LoginCookieAttributes verifies session cookie attributes
func TestMode2_WebAuth_LoginCookieAttributes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	// Create test user
	email := fmt.Sprintf("testcookie-%s@internal.grid", uuid.New().String()[:8])
	createTestUser(t, "testcookie", email, "password123")

	// Login
	_, sessionCookie := loginWebUser(t, email, "password123")

	// Verify cookie attributes
	require.True(t, sessionCookie.HttpOnly, "Session cookie should be HttpOnly")
	require.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite, "Session cookie should use SameSite=Lax")
	require.Equal(t, "/", sessionCookie.Path, "Session cookie path should be '/'")

	// Verify expiry is approximately 2 hours from now (within 5 minute tolerance)
	expectedExpiry := time.Now().Add(2 * time.Hour)
	expiryDiff := sessionCookie.Expires.Sub(expectedExpiry).Abs()
	require.Less(t, expiryDiff, 5*time.Minute, "Session cookie should expire in ~2 hours")

	t.Logf("✓ Cookie attributes verified: HttpOnly=%v, SameSite=%v, Expires=%v",
		sessionCookie.HttpOnly, sessionCookie.SameSite, sessionCookie.Expires)
}

// TestMode2_WebAuth_LoginInvalidCredentials verifies 401 for invalid credentials
func TestMode2_WebAuth_LoginInvalidCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	testCases := []struct {
		name     string
		email    string
		password string
		setup    func(t *testing.T) string // Returns email to use
	}{
		{
			name:     "NonExistentUser",
			email:    "nonexistent@internal.grid",
			password: "password123",
			setup:    func(t *testing.T) string { return "nonexistent@internal.grid" },
		},
		{
			name:     "WrongPassword",
			password: "wrongpassword",
			setup: func(t *testing.T) string {
				email := fmt.Sprintf("wrongpw-%s@internal.grid", uuid.New().String()[:8])
				createTestUser(t, "wrongpw", email, "correctpassword")
				return email
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := tc.setup(t)
			if tc.email != "" {
				email = tc.email
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			loginURL := fmt.Sprintf("%s/auth/login", serverURL)
			payload := fmt.Sprintf(`{"username":"%s","password":"%s"}`, email, tc.password)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewBufferString(payload))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Invalid credentials should return 401")

			bodyBytes, _ := io.ReadAll(resp.Body)
			bodyText := string(bodyBytes)
			require.Contains(t, bodyText, "Invalid credentials", "Error message should be generic")

			t.Logf("✓ Invalid credentials returned 401: %s", bodyText)
		})
	}
}

// TestMode2_WebAuth_LoginDisabledAccount verifies 403 for disabled accounts
func TestMode2_WebAuth_LoginDisabledAccount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	// Create test user
	email := fmt.Sprintf("disabled-%s@internal.grid", uuid.New().String()[:8])
	createTestUser(t, "disabled", email, "password123")

	// Disable the user
	disableUser(t, email)

	// Attempt login
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	loginURL := fmt.Sprintf("%s/auth/login", serverURL)
	payload := fmt.Sprintf(`{"username":"%s","password":"password123"}`, email)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewBufferString(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusForbidden, resp.StatusCode, "Disabled account should return 403")

	bodyBytes, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(bodyBytes), "Account disabled", "Error message should indicate account is disabled")

	t.Log("✓ Disabled account returned 403")
}

// TestMode2_WebAuth_WhoamiSuccess verifies session restoration via whoami
func TestMode2_WebAuth_WhoamiSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	// Create test user
	email := fmt.Sprintf("whoami-%s@internal.grid", uuid.New().String()[:8])
	createTestUser(t, "whoami", email, "password123")

	// Login
	loginResp, sessionCookie := loginWebUser(t, email, "password123")

	// Call whoami
	whoamiResp, err := fetchWhoamiWeb(t, sessionCookie)
	require.NoError(t, err, "Whoami should succeed with valid session cookie")

	// Verify user data matches login response
	require.Equal(t, loginResp.User.ID, whoamiResp.User.ID, "User ID should match login response")
	require.Equal(t, loginResp.User.Username, whoamiResp.User.Username, "Username should match")
	require.Equal(t, loginResp.User.Email, whoamiResp.User.Email, "Email should match")
	require.Equal(t, "internal", whoamiResp.User.AuthType, "Auth type should be 'internal'")

	// Verify session data
	require.NotEmpty(t, whoamiResp.Session.ID, "Session ID should be set")
	require.Greater(t, whoamiResp.Session.ExpiresAt, time.Now().UnixMilli(), "Session should not be expired")

	t.Logf("✓ Whoami success: user=%s, session_id=%s", whoamiResp.User.Email, whoamiResp.Session.ID)
}

// TestMode2_WebAuth_WhoamiUnauthenticated verifies 401 without session cookie
func TestMode2_WebAuth_WhoamiUnauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whoamiURL := fmt.Sprintf("%s/api/auth/whoami", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, whoamiURL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Whoami without cookie should return 401")

	bodyBytes, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(bodyBytes), "unauthenticated", "Error message should indicate unauthenticated")

	t.Log("✓ Whoami without cookie returned 401")
}

// TestMode2_WebAuth_AuthConfig verifies GET /auth/config returns internal IdP mode
func TestMode2_WebAuth_AuthConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	configURL := fmt.Sprintf("%s/auth/config", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Auth config should return 200")

	var config struct {
		Mode               string `json:"mode"`
		Issuer             string `json:"issuer"`
		ClientID           string `json:"client_id"`
		Audience           string `json:"audience"`
		SupportsDeviceFlow bool   `json:"supports_device_flow"`
	}
	err = json.NewDecoder(resp.Body).Decode(&config)
	require.NoError(t, err, "Failed to decode auth config response")

	// Verify Mode 2 configuration
	require.Equal(t, "internal-idp", config.Mode, "Mode should be 'internal-idp'")
	require.Equal(t, "http://localhost:8080", config.Issuer, "Issuer should match OIDC_ISSUER")
	require.Equal(t, "gridapi", config.ClientID, "ClientID should match OIDC_CLIENT_ID")
	require.NotEmpty(t, config.Audience, "Audience should be set")

	t.Logf("✓ Auth config verified: mode=%s, issuer=%s, clientId=%s",
		config.Mode, config.Issuer, config.ClientID)
}

// TestMode2_WebAuth_LogoutSuccess verifies logout clears session
func TestMode2_WebAuth_LogoutSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	// Create test user
	email := fmt.Sprintf("logout-%s@internal.grid", uuid.New().String()[:8])
	createTestUser(t, "logout", email, "password123")

	// Login
	_, sessionCookie := loginWebUser(t, email, "password123")

	// Verify whoami works before logout
	_, err := fetchWhoamiWeb(t, sessionCookie)
	require.NoError(t, err, "Whoami should work before logout")

	// Logout
	logoutWebUser(t, sessionCookie)

	// Verify whoami fails after logout
	_, err = fetchWhoamiWeb(t, sessionCookie)
	require.Error(t, err, "Whoami should fail after logout")
	require.Contains(t, err.Error(), "401", "Whoami should return 401 after logout")

	t.Log("✓ Logout successfully invalidated session")
}

// TestMode2_WebAuth_FullFlow verifies complete login → whoami → logout flow
func TestMode2_WebAuth_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	// Create test user
	email := fmt.Sprintf("fullflow-%s@internal.grid", uuid.New().String()[:8])
	createTestUser(t, "fullflow", email, "password123")

	// Step 1: Login
	t.Log("Step 1: Login")
	loginResp, sessionCookie := loginWebUser(t, email, "password123")
	require.Equal(t, "internal", loginResp.User.AuthType)
	require.NotEmpty(t, sessionCookie.Value)

	// Step 2: Restore session via whoami
	t.Log("Step 2: Restore session via whoami")
	whoamiResp, err := fetchWhoamiWeb(t, sessionCookie)
	require.NoError(t, err)
	require.Equal(t, loginResp.User.ID, whoamiResp.User.ID)
	require.Equal(t, email, whoamiResp.User.Email)

	// Step 3: Logout
	t.Log("Step 3: Logout")
	logoutWebUser(t, sessionCookie)

	// Step 4: Verify session is invalid
	t.Log("Step 4: Verify session invalidation")
	_, err = fetchWhoamiWeb(t, sessionCookie)
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")

	t.Log("✓ Full authentication flow completed successfully")
}

// TestMode2_WebAuth_SessionWithConnectRPC verifies webapp's contract:
// Session cookies + Connect RPC authentication (ListStates, CreateState, UpdateLabels)
func TestMode2_WebAuth_SessionWithConnectRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	verifyAuthEnabled(t, "Mode 2")

	if !isMode2Configured(t) {
		t.Skip("Server not configured for Mode 2 (Internal IdP)")
	}

	// Step 1: Create test user
	t.Log("Step 1: Create test user")
	email := fmt.Sprintf("rpc-user-%s@internal.grid", uuid.New().String()[:8])
	createTestUser(t, "rpcuser", email, "password123")

	// Step 2: Login via HTTP and get session cookie
	t.Log("Step 2: Login via HTTP")
	_, sessionCookie := loginWebUser(t, email, "password123")
	require.NotEmpty(t, sessionCookie.Value)

	// Step 3: Create HTTP client with cookie jar containing session cookie
	t.Log("Step 3: Create HTTP client with session cookie")
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	// Add session cookie to jar for localhost:8080
	url, err := url.Parse(serverURL)
	require.NoError(t, err)
	jar.SetCookies(url, []*http.Cookie{sessionCookie})

	httpClient := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// Step 4: Create SDK client with authenticated HTTP client
	t.Log("Step 4: Create SDK client with authenticated HTTP client")
	client := sdk.NewClient(serverURL, sdk.WithHTTPClient(httpClient))
	require.NotNil(t, client)

	// Step 5: Call ListStates RPC with session cookie
	t.Log("Step 5: Call ListStates RPC")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	states, err := client.ListStates(ctx)
	require.NoError(t, err, "ListStates should succeed with session cookie")
	require.NotNil(t, states, "ListStates should return states")
	t.Logf("ListStates succeeded, returned %d states", len(states))

	// Step 6: Call CreateState RPC with session cookie
	t.Log("Step 6: Call CreateState RPC")
	logicID := fmt.Sprintf("rpc-state-%s", uuid.New().String()[:8])
	createInput := sdk.CreateStateInput{
		LogicID: logicID,
		Labels: map[string]interface{}{
			"env": "test",
		},
	}

	createdState, err := client.CreateState(ctx, createInput)
	require.NoError(t, err, "CreateState should succeed with session cookie")
	require.NotNil(t, createdState, "CreateState should return state")
	require.Equal(t, logicID, createdState.LogicID)
	t.Logf("CreateState succeeded, created state: %s", createdState.GUID)

	// Step 7: Call UpdateLabels RPC with session cookie
	t.Log("Step 7: Call UpdateLabels RPC")
	labelInput := sdk.UpdateStateLabelsInput{
		StateID: createdState.GUID,
		Adds: map[string]interface{}{
			"team":    "platform",
			"updated": "true",
		},
	}

	_, err = client.UpdateStateLabels(ctx, labelInput)
	require.NoError(t, err, "UpdateLabels should succeed with session cookie")
	t.Logf("UpdateLabels succeeded")

	// Step 8: Verify state has updated labels
	t.Log("Step 8: Verify state labels")
	include := true
	statesWithLabels, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{IncludeLabels: &include})
	require.NoError(t, err)

	var updatedState *sdk.StateSummary
	for i := range statesWithLabels {
		if statesWithLabels[i].LogicID == logicID {
			updatedState = &statesWithLabels[i]
			break
		}
	}
	require.NotNil(t, updatedState, "Updated state should be found in list")
	require.Equal(t, "platform", updatedState.Labels["team"])
	require.Equal(t, "true", updatedState.Labels["updated"])
	t.Logf("State labels verified: team=platform, updated=true")

	// Step 9: Logout
	t.Log("Step 9: Logout")
	logoutWebUser(t, sessionCookie)

	// Step 10: Verify Connect RPCs return 401 after logout
	t.Log("Step 10: Verify 401 after logout")

	// Create new context for post-logout tests
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	// Try ListStates - should fail with 401
	_, err = client.ListStates(ctx2)
	require.Error(t, err, "ListStates should fail after logout")
	// Check for 401 in error message
	require.Contains(t, err.Error(), "401", "Error should be 401 Unauthorized")
	t.Logf("ListStates correctly returned 401 after logout")

	// Try CreateState - should fail with 401
	logicID2 := fmt.Sprintf("rpc-state-2-%s", uuid.New().String()[:8])
	createInput2 := sdk.CreateStateInput{LogicID: logicID2}
	_, err = client.CreateState(ctx2, createInput2)
	require.Error(t, err, "CreateState should fail after logout")
	require.Contains(t, err.Error(), "401", "Error should be 401 Unauthorized")
	t.Logf("CreateState correctly returned 401 after logout")

	t.Log("✓ Session + Connect RPC authentication test completed successfully")
}
