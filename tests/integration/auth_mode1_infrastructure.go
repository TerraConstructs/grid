package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ===============================================
// Mode 1 (External IdP) Constants
// ===============================================

const (
	keycloakBaseURL   = "http://localhost:8443"
	keycloakRealm     = "grid"
	keycloakTokenPath = "/realms/grid/protocol/openid-connect/token"
	keycloakDiscovery = "/realms/grid/.well-known/openid-configuration"
)

// ===============================================
// Infrastructure Helpers
// ===============================================

// isMode1Configured checks if the server is running in Mode 1 (External IdP)
func isMode1Configured(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// In Mode 1, Grid does NOT expose OIDC discovery (it's a Resource Server, not IdP)
	// Grid should NOT have /.well-known/openid-configuration
	gridDiscoveryURL := fmt.Sprintf("%s/.well-known/openid-configuration", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gridDiscoveryURL, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Grid should return 404 for OIDC discovery in Mode 1
	if resp.StatusCode == http.StatusOK {
		return false
	}

	// Check if Grid exposes SSO endpoints (Mode 1 RelyingParty endpoints)
	ssoLoginURL := fmt.Sprintf("%s/auth/sso/login", serverURL)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, ssoLoginURL, nil)
	if err != nil {
		return false
	}

	// Don't follow redirects - we want to see the redirect response itself
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err = noRedirectClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Should redirect to Keycloak in Mode 1
	return resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect
}

// isKeycloakHealthy checks if Keycloak is running and accessible
func isKeycloakHealthy(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check Keycloak by accessing the realm endpoint
	// Keycloak 22 doesn't expose /health/ready without auth
	realmURL := fmt.Sprintf("%s/realms/%s", keycloakBaseURL, keycloakRealm)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, realmURL, nil)
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

// keycloakDiscoveryResponse represents the OIDC discovery document from Keycloak
type keycloakDiscoveryResponse struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint"`
	GrantTypesSupported   []string `json:"grant_types_supported"`
}

// getKeycloakDiscovery fetches the OIDC discovery document from Keycloak
func getKeycloakDiscovery(t *testing.T) *keycloakDiscoveryResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	discoveryURL := fmt.Sprintf("%s%s", keycloakBaseURL, keycloakDiscovery)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	require.NoError(t, err, "Failed to create discovery request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to fetch Keycloak discovery")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Keycloak discovery should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read discovery response")

	var discovery keycloakDiscoveryResponse
	err = json.Unmarshal(body, &discovery)
	require.NoError(t, err, "Failed to parse discovery document")

	return &discovery
}
