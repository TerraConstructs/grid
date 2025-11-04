// pkg/sdk/auth.go
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/client/rp/cli"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// LoginSuccessMetadata contains information about the successful login,
// useful for displaying a confirmation message to the user.
type LoginSuccessMetadata struct {
	// User is the 'sub' claim from the ID token.
	User string
	// Email is the 'email' claim, if present.
	Email string
	// ExpiresAt is when the access token expires.
	ExpiresAt time.Time
}

// AuthConfig represents Grid's authentication configuration.
// This is returned by the /auth/config endpoint and tells SDK clients
// where to authenticate and what client ID to use.
type AuthConfig struct {
	Mode               string  `json:"mode"`                 // "external-idp" or "internal-idp"
	Issuer             string  `json:"issuer"`               // OIDC issuer URL
	ClientID           *string `json:"client_id,omitempty"`  // Public client ID for device flow (nil if not supported)
	Audience           *string `json:"audience,omitempty"`   // Expected aud claim in access tokens
	SupportsDeviceFlow bool    `json:"supports_device_flow"` // Whether interactive device flow is supported
}

// DiscoverAuthConfig fetches authentication configuration from Grid API.
// This enables mode-agnostic authentication by discovering whether Grid
// is using an external IdP (Mode 1) or internal IdP (Mode 2).
func DiscoverAuthConfig(ctx context.Context, serverURL string) (*AuthConfig, error) {
	url := fmt.Sprintf("%s/auth/config", serverURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var config AuthConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse auth config: %w", err)
	}

	return &config, nil
}

// LoginInteractive performs device flow authentication using discovered config.
// This is the SDK's main authentication entrypoint for interactive users.
//
// Flow:
// 1. Discovers auth configuration from Grid API (/auth/config)
// 2. Checks if interactive device flow is supported
// 3. Initiates OIDC device authorization flow against the discovered issuer
// 4. Optionally saves credentials using the provided store
//
// The store parameter is optional - pass nil to skip credential persistence.
//
// Returns an error if the Grid deployment does not support interactive authentication
// (e.g., Mode 2 Internal IdP only supports service accounts).
func LoginInteractive(ctx context.Context, serverURL string, store CredentialStore) (*LoginSuccessMetadata, error) {
	// 1. Discover auth config
	config, err := DiscoverAuthConfig(ctx, serverURL)
	if err != nil {
		return nil, fmt.Errorf("auth discovery failed: %w", err)
	}

	// 2. Check if device flow is supported
	if !config.SupportsDeviceFlow {
		return nil, fmt.Errorf("interactive login not supported in mode=%s (use service account authentication with GRID_CLIENT_ID and GRID_CLIENT_SECRET)", config.Mode)
	}

	if config.ClientID == nil || *config.ClientID == "" {
		return nil, fmt.Errorf("no client_id returned for device flow")
	}

	// 3. Perform device flow
	meta, creds, err := performDeviceFlow(ctx, config.Issuer, *config.ClientID)
	if err != nil {
		return nil, err
	}

	// 4. Save credentials (if store provided)
	if store != nil {
		if err := store.SaveCredentials(creds); err != nil {
			return nil, fmt.Errorf("failed to save credentials: %w", err)
		}
	}

	return meta, nil
}

// LoginWithDeviceCode initiates the OIDC Device Authorization Flow (RFC 8628).
// It guides the user to authorize the CLI in a browser, polls for tokens,
// and returns the credentials.
//
// Deprecated: Use LoginInteractive instead, which automatically discovers
// the correct issuer and client ID from the Grid API.
//
// This function works with both Grid deployment modes:
//   - Mode 1 (External IdP): issuer = external IdP URL (e.g., https://keycloak.local/realms/grid)
//   - Mode 2 (Internal IdP): issuer = Grid's URL (e.g., https://grid.example.com)
//
// The function performs OIDC discovery from the issuer to find device authorization
// endpoints automatically.
func LoginWithDeviceCode(
	ctx context.Context,
	issuer string,
	clientID string,
) (*LoginSuccessMetadata, *Credentials, error) {
	return performDeviceFlow(ctx, issuer, clientID)
}

// performDeviceFlow is the internal implementation of device authorization flow.
// It initiates the OIDC Device Authorization Flow (RFC 8628), guides the user
// to authorize in a browser, and polls for tokens.
func performDeviceFlow(
	ctx context.Context,
	issuer string,
	clientID string,
) (*LoginSuccessMetadata, *Credentials, error) {

	scopes := []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail, oidc.ScopeOfflineAccess}

	// 1. Discover Provider Configuration
	// The relying party client performs OIDC discovery (/.well-known/openid-configuration)
	// This works for both Grid as provider (Mode 2) and external IdP (Mode 1)
	relyingParty, err := rp.NewRelyingPartyOIDC(
		ctx,
		issuer,
		clientID,
		"", // clientSecret - empty for public client (device flow)
		"", // redirectURI - not used for device flow
		scopes,
		rp.WithHTTPClient(defaultHTTPClient()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to discover OIDC provider at %s: %w", issuer, err)
	}

	// 2. Start the Device Authorization Flow
	// Calls the /device_authorization endpoint discovered from the provider
	// Returns: DeviceCode, UserCode, VerificationURI, VerificationURIComplete, Interval
	authResponse, err := rp.DeviceAuthorization(ctx, scopes, relyingParty, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start device authorization flow: %w", err)
	}

	// 3. Display User Instructions
	printDeviceCodeInstructions(authResponse)

	// Attempt to open browser automatically (best effort)
	// OpenBrowser doesn't return an error - it just tries to open the browser
	if authResponse.VerificationURIComplete != "" {
		cli.OpenBrowser(authResponse.VerificationURIComplete)
		log.Println("Attempted to open browser automatically")
	}

	// 4. Poll the Token Endpoint
	// Polls /token endpoint with device_code grant type until user approves or times out
	// The interval is provided by the authorization server (typically 5 seconds)
	interval := time.Duration(authResponse.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second // Default if not provided
	}

	token, err := rp.DeviceAccessToken(ctx, authResponse.DeviceCode, interval, relyingParty)
	if err != nil {
		return nil, nil, fmt.Errorf("device authorization failed: %w\n\nThis usually means:\n  - User denied the request\n  - Authorization expired (timeout)\n  - Network connectivity issues", err)
	}

	// 5. Extract ID Token Claims (if available)
	var idTokenClaims *oidc.IDTokenClaims
	if token.IDToken != "" {
		// Parse ID token to extract claims
		claims, err := rp.VerifyIDToken[*oidc.IDTokenClaims](ctx, token.IDToken, relyingParty.IDTokenVerifier())
		if err != nil {
			log.Printf("Warning: failed to verify ID token: %v", err)
		} else {
			idTokenClaims = claims
		}
	}

	// 6. Persist the Credentials
	// Calculate expiry time from ExpiresIn (seconds from now)
	expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	creds := &Credentials{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    expiresAt,
	}

	// Store principal ID if we have ID token claims
	if idTokenClaims != nil {
		creds.PrincipalID = "user:" + idTokenClaims.Subject
	}

	// 7. Return Success Metadata
	metadata := &LoginSuccessMetadata{
		ExpiresAt: expiresAt,
	}
	if idTokenClaims != nil {
		metadata.User = idTokenClaims.Subject
		metadata.Email = idTokenClaims.Email
	}

	return metadata, creds, nil
}

// LoginWithServiceAccount authenticates using the OAuth2 client credentials flow.
// This is used for service accounts (machine-to-machine authentication).
//
// This function works with both Grid deployment modes:
//   - Mode 1 (External IdP): Service account is created in external IdP (e.g., Keycloak client)
//   - Mode 2 (Internal IdP): Service account is created in Grid via CreateServiceAccount RPC
//
// The function performs OIDC discovery to find the token endpoint, then exchanges
// client credentials for an access token.
func LoginWithServiceAccount(
	ctx context.Context,
	issuer string,
	clientID string,
	clientSecret string,
) (*Credentials, error) {
	// 1. Discover Provider Configuration
	// We only need discovery to get the token endpoint URL
	scopes := []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}
	discoverer, err := rp.NewRelyingPartyOIDC(
		ctx,
		issuer,
		clientID,
		clientSecret,
		"", // redirectURI - not used for client credentials flow
		scopes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider at %s: %w", issuer, err)
	}

	// 2. Configure Client Credentials Flow
	// Use golang.org/x/oauth2 for client credentials grant
	ccConfig := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     discoverer.OAuthConfig().Endpoint.TokenURL,
		Scopes:       scopes,
	}

	// 3. Exchange Credentials for Token
	// Calls POST /token with grant_type=client_credentials
	token, err := ccConfig.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange client credentials for token: %w", err)
	}

	// 4. Persist the Credentials
	creds := &Credentials{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		ExpiresAt:    token.Expiry,
		RefreshToken: token.RefreshToken, // Usually empty for client credentials
		PrincipalID:  "sa:" + clientID,   // Service account principal ID
	}

	return creds, nil
}

// RefreshToken attempts to refresh an expired access token using a refresh token.
// Returns the new credentials on success.
func RefreshToken(
	ctx context.Context,
	issuer string,
	clientID string,
	refreshToken string,
) (*Credentials, error) {
	// Discover provider to get token endpoint
	scopes := []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail, oidc.ScopeOfflineAccess}
	relyingParty, err := rp.NewRelyingPartyOIDC(
		ctx,
		issuer,
		clientID,
		"",
		"",
		scopes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

	// Perform token refresh using oauth2 library
	oauthConfig := relyingParty.OAuthConfig()
	tokenSource := oauthConfig.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	})

	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Update stored credentials
	creds := &Credentials{
		AccessToken:  newToken.AccessToken,
		TokenType:    newToken.TokenType,
		RefreshToken: newToken.RefreshToken,
		ExpiresAt:    newToken.Expiry,
	}

	return creds, nil
}

// --- Helper Functions ---

// defaultHTTPClient returns an HTTP client with reasonable timeout for OIDC operations.
func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

// printDeviceCodeInstructions displays the device code and verification URL to the user.
func printDeviceCodeInstructions(authResponse *oidc.DeviceAuthorizationResponse) {
	fmt.Println("============================================================")
	fmt.Printf("Your user code is: %s\n", authResponse.UserCode)
	fmt.Println("")
	fmt.Println("Please visit the following URL in your browser to authorize this device:")
	fmt.Printf("  %s\n", authResponse.VerificationURI)
	fmt.Println("")
	if authResponse.VerificationURIComplete != "" {
		fmt.Println("Or use this direct link (includes code):")
		fmt.Printf("  %s\n", authResponse.VerificationURIComplete)
	}
	fmt.Println("============================================================")
	fmt.Println("Waiting for authorization...")
	fmt.Println("")
}

type EnvCreds struct {
	ClientID     string
	ClientSecret string
}

func CheckEnvCreds() (bool, EnvCreds) {
	hasEnvCreds := os.Getenv("GRID_CLIENT_ID") != "" &&
		os.Getenv("GRID_CLIENT_SECRET") != ""
	creds := EnvCreds{
		ClientID:     os.Getenv("GRID_CLIENT_ID"),
		ClientSecret: os.Getenv("GRID_CLIENT_SECRET"),
	}
	return hasEnvCreds, creds
}
