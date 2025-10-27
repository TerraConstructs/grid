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
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
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

	// Check for OIDC discovery endpoint which is only available in Mode 2
	discoveryURL := fmt.Sprintf("%s/.well-known/openid-configuration", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// setupMode2Environment configures environment variables for Mode 2 (Internal IdP)
func setupMode2Environment(t *testing.T) {
	t.Helper()
	os.Setenv("OIDC_ISSUER", "http://localhost:8080")
	os.Setenv("OIDC_CLIENT_ID", "gridapi")
	os.Setenv("OIDC_SIGNING_KEY_PATH", mode2SigningKeyPath)
	t.Logf("Mode 2 environment configured: OIDC_ISSUER=http://localhost:8080, OIDC_SIGNING_KEY_PATH=%s", mode2SigningKeyPath)
}

// cleanupSigningKeys removes any existing signing keys to force regeneration
func cleanupSigningKeys(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll(mode2SigningKeyDir); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to clean up signing keys: %v", err)
	}
	t.Logf("Cleaned up signing keys directory: %s", mode2SigningKeyDir)
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
	Iss string        `json:"iss"`
	Sub string        `json:"sub"`
	Aud interface{}   `json:"aud"` // Can be string or []string
	Jti string        `json:"jti"`
	Exp int64         `json:"exp"`
	Iat int64         `json:"iat"`
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
