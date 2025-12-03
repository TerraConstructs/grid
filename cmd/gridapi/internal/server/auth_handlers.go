package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// HandleSSOLogin initiates the OIDC Authorization Code Flow.
// Accepts optional redirect_uri query parameter to specify where to redirect after successful authentication.
// Related: Beads issue grid-202d (SSO callback redirect fix)
func HandleSSOLogin(rpAuth *auth.RelyingParty) http.HandlerFunc {
	// Create the library's AuthURLHandler once. It handles PKCE challenge generation/storage and redirects.
	// We pass a state generator function to it.
	libraryAuthHandler := rp.AuthURLHandler(func() string {
		state, _ := auth.GenerateNonce() // Reusing GenerateNonce for a secure random string for OIDC state
		return state
	}, rpAuth.RP()) // Use InnerRP() to pass the actual zitadel/oidc RelyingParty instance
	return func(w http.ResponseWriter, r *http.Request) {
		// Store redirect_uri from query parameter if provided
		// This will be used after OAuth callback to redirect back to the webapp.
		// This cookie is separate from OIDC state/PKCE cookies managed by the library.
		if redirectURI := r.URL.Query().Get("redirect_uri"); redirectURI != "" {
			auth.SetRedirectURICookie(w, r, redirectURI)
		}
		// Delegate to the library's AuthURLHandler. It will handle:
		// 1. Generating code_challenge (if PKCE is enabled in rpAuth.InnerRP()).
		// 2. Storing code_verifier in a cookie (handled by rpAuth.InnerRP()'s CookieHandler).
		// 3. Storing the OIDC state in a cookie (handled by rpAuth.InnerRP()'s CookieHandler).
		// 4. Redirecting the user to the Identity Provider's authorization endpoint.
		libraryAuthHandler.ServeHTTP(w, r)
	}
}

// HandleSSOCallback handles the OIDC callback, exchanges the code for a token,
// verifies the token, and establishes a session.
func HandleSSOCallback(rpAuth *auth.RelyingParty, iamService iamAdminService) http.HandlerFunc {

	// Define the callback function that will be executed after a successful token exchange by CodeExchangeHandler
	codeExchangeCallback := func(w http.ResponseWriter, r *http.Request, tokens *oidc.Tokens[*oidc.IDTokenClaims], state string, provider rp.RelyingParty) {
		ctx := r.Context()
		// The library's CodeExchangeHandler has already validated the OIDC 'state' parameter
		// and handled the PKCE verifier exchange.
		idTokenClaims := tokens.IDTokenClaims
		rawIDToken := tokens.IDToken
		// Get or create user (JIT provisioning via IAM service)
		user, err := iamService.GetUserBySubject(ctx, idTokenClaims.Subject)
		if err != nil {
			// User not found, create a new one via IAM service (with subject for external IdP)
			user, err = iamService.CreateUser(ctx, idTokenClaims.Email, idTokenClaims.Name, idTokenClaims.Subject, "")
			if err != nil {
				log.Printf("SSO callback: failed to create user (subject=%s, email=%s): %v",
					idTokenClaims.Subject, idTokenClaims.Email, err)
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
		}
		// Create session via IAM service
		_, token, err := iamService.CreateSession(ctx, user.ID, rawIDToken, tokens.Expiry)
		if err != nil {
			log.Printf("SSO callback: failed to create session (user_id=%s): %v", user.ID, err)
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
		// Set the session cookie for gridapi
		cookie := &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    token,
			Path:     "/",
			Expires:  tokens.Expiry,
			HttpOnly: true,
			Secure:   r.URL.Scheme == "https",
			SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(w, cookie)
		// Redirect to the URI specified in the original login request (from cookie)
		// Defaults to "/" if not provided. This enables webapp dev mode to work correctly.
		// Related: Beads issue grid-202d (SSO callback redirect fix)
		redirectURI := auth.GetRedirectURICookie(w, r)
		if redirectURI == "" {
			redirectURI = "/"
		}
		http.Redirect(w, r, redirectURI, http.StatusFound)
	}
	// Return the library's CodeExchangeHandler. It will handle:
	// 1. Reading the OIDC state cookie and validating it.
	// 2. Reading the PKCE verifier cookie and passing it to the token endpoint.
	// 3. Exchanging the authorization code for tokens.
	// 4. Invoking our custom `codeExchangeCallback` on success.
	// 5. Handling errors during exchange (e.g., "invalid_grant" from IdP).
	return rp.CodeExchangeHandler(codeExchangeCallback, rpAuth.RP())
}

// HandleLogout revokes the user's session.
func HandleLogout(iamService iamAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.GetUserFromContext(r.Context())
		if !ok {
			http.Error(w, "No active session", http.StatusUnauthorized)
			return
		}

		// Revoke session via IAM service
		if err := iamService.RevokeSession(r.Context(), principal.SessionID); err != nil {
			http.Error(w, "Failed to revoke session", http.StatusInternalServerError)
			return
		}

		// Clear the session cookie
		cookie := &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   r.URL.Scheme == "https",
			SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(w, cookie)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Logged out"))
	}
}

// AuthConfigResponse tells SDK clients how to authenticate
type AuthConfigResponse struct {
	Mode               string  `json:"mode"`                 // "external-idp" or "internal-idp"
	Issuer             string  `json:"issuer"`               // OIDC issuer URL
	ClientID           *string `json:"client_id,omitempty"`  // Public client ID for device flow (Mode 1 only)
	Audience           *string `json:"audience,omitempty"`   // Expected aud claim in access tokens
	SupportsDeviceFlow bool    `json:"supports_device_flow"` // Whether interactive device flow is supported
}

// HandleAuthConfig returns the authentication configuration for SDK clients.
// This endpoint enables mode-agnostic authentication discovery, allowing the SDK
// to automatically determine whether to authenticate against an external IdP
// (Mode 1) or Grid's internal IdP (Mode 2).
//
// Mode 1 (External IdP): Supports interactive device flow for human users
// Mode 2 (Internal IdP): Only supports service account authentication (no device flow)
func HandleAuthConfig(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := AuthConfigResponse{}

		if cfg.OIDC.ExternalIdP != nil {
			// Mode 1: External IdP (e.g., Keycloak, Azure Entra ID, Okta)
			// Supports interactive device flow for CLI users
			response.Mode = "external-idp"
			response.Issuer = cfg.OIDC.ExternalIdP.Issuer
			response.ClientID = &cfg.OIDC.ExternalIdP.CLIClientID
			gridAPIAudience := "grid-api"
			response.Audience = &gridAPIAudience
			response.SupportsDeviceFlow = true
		} else if cfg.OIDC.Issuer != "" {
			// Mode 2: Internal IdP (Grid acts as OIDC provider)
			// Only supports service account authentication (client credentials)
			response.Mode = "internal-idp"
			response.Issuer = cfg.OIDC.Issuer
			response.ClientID = &cfg.OIDC.ClientID // Return clientID for webapp authentication
			response.Audience = &cfg.OIDC.ClientID
			response.SupportsDeviceFlow = false
		} else {
			http.Error(w, "Authentication not configured", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

// InternalLoginRequest represents credentials for internal IdP authentication
type InternalLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UserResponse represents user data in API responses
type UserResponse struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	AuthType string   `json:"auth_type"`
	Roles    []string `json:"roles"`
	Groups   []string `json:"groups,omitempty"` // Group memberships (external IdP only)
}

// LoginResponse represents the response from POST /auth/login
type LoginResponse struct {
	User      UserResponse `json:"user"`
	ExpiresAt int64        `json:"expires_at"`
}

// SessionResponse represents session data in API responses
type SessionResponse struct {
	ID        string `json:"id"`
	ExpiresAt int64  `json:"expires_at"`
}

// WhoamiResponse represents the response from GET /api/auth/whoami
type WhoamiResponse struct {
	User    UserResponse    `json:"user"`
	Session SessionResponse `json:"session"`
}

// HandleInternalLogin authenticates users with username/password for internal IdP mode
func HandleInternalLogin(iamService iamAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Parse request body
		var req InternalLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate input
		if req.Username == "" || req.Password == "" {
			http.Error(w, "Missing username or password", http.StatusBadRequest)
			return
		}

		// Lookup user by email (via IAM service)
		user, err := iamService.GetUserByEmail(ctx, req.Username)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Verify password hash
		if user.PasswordHash == nil || *user.PasswordHash == "" {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		if err := verifyPasswordHash(*user.PasswordHash, req.Password); err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Check if user is disabled
		if user.DisabledAt != nil {
			http.Error(w, "Account disabled", http.StatusForbidden)
			return
		}

		// Create session via IAM service
		expiresAt := time.Now().Add(2 * time.Hour)
		session, token, err := iamService.CreateSession(ctx, user.ID, "", expiresAt)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		// Resolve roles for internal IdP (no groups, isUser=true)
		roles, err := iamService.ResolveRoles(ctx, user.ID, []string{}, true)
		if err != nil {
			roles = []string{}
		}

		// Set session cookie
		cookie := &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    token,
			Path:     "/",
			Expires:  expiresAt,
			HttpOnly: true,
			Secure:   r.URL.Scheme == "https",
			SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(w, cookie)

		// Return login response
		w.Header().Set("Content-Type", "application/json")
		resp := LoginResponse{
			User: UserResponse{
				ID:       user.ID,
				Username: user.Name,
				Email:    user.Email,
				AuthType: "internal",
				Roles:    roles,
				Groups:   []string{}, // Internal IdP users don't have groups
			},
			ExpiresAt: expiresAt.UnixMilli(),
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}

		// Unused session variable
		_ = session
	}
}

// HandleWhoAmI returns the authenticated user's information and session metadata
func HandleWhoAmI(iamService iamAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract principal from context (set by authn middleware)
		principal, ok := auth.GetUserFromContext(ctx)
		if !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}

		// Fetch session via IAM service
		session, err := iamService.GetSessionByID(ctx, principal.SessionID)
		if err != nil || session == nil {
			http.Error(w, "Session not found", http.StatusUnauthorized)
			return
		}

		// Fetch user via IAM service
		user, err := iamService.GetUserByID(ctx, principal.InternalID)
		if err != nil {
			http.Error(w, "User not found", http.StatusInternalServerError)
			return
		}

		// Determine auth type
		authType := "internal"
		isExternalIdP := user.Subject != nil && *user.Subject != ""
		if isExternalIdP {
			authType = "external"
		}

		// Extract groups from session ID token (for external IdP users)
		// This is critical for group→role mapping to work correctly
		groups, err := auth.ExtractGroupsFromIDToken(session.IDToken)
		if err != nil {
			// Failed to extract groups, continue with empty groups
			// This is not a fatal error - user may have no groups
			groups = []string{}
		}

		// Resolve roles via IAM service with groups
		// For external IdP users, this uses the group→role cache to map groups to roles
		roles, err := iamService.ResolveRoles(ctx, user.ID, groups, true /* isUser */)
		if err != nil {
			roles = []string{}
		}

		// Build response
		w.Header().Set("Content-Type", "application/json")
		resp := WhoamiResponse{
			User: UserResponse{
				ID:       user.ID,
				Username: user.Name,
				Email:    user.Email,
				AuthType: authType,
				Roles:    roles,
				Groups:   groups,
			},
			Session: SessionResponse{
				ID:        session.ID,
				ExpiresAt: session.ExpiresAt.UnixMilli(),
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

// verifyPasswordHash checks if the provided password matches the bcrypt hash
func verifyPasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
