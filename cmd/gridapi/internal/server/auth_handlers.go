package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	gridmiddleware "github.com/terraconstructs/grid/cmd/gridapi/internal/middleware"
)

// HandleSSOLogin initiates the OIDC Authorization Code Flow.
func HandleSSOLogin(rp *auth.RelyingParty) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := auth.GenerateNonce()
		if err != nil {
			http.Error(w, "Failed to generate state", http.StatusInternalServerError)
			return
		}
		auth.SetStateCookie(w, state)
		http.Redirect(w, r, rp.AuthCodeURL(state), http.StatusFound)
	}
}

// HandleSSOCallback handles the OIDC callback, exchanges the code for a token,
// verifies the token, and establishes a session.
func HandleSSOCallback(rp *auth.RelyingParty, deps *gridmiddleware.AuthnDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if err := auth.VerifyStateCookie(r, r.URL.Query().Get("state")); err != nil {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}

		tokens, err := rp.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "Failed to exchange code for token", http.StatusUnauthorized)
			return
		}

		idTokenClaims := tokens.IDTokenClaims
		rawIDToken := tokens.IDToken

		// Get or create user
		user, err := deps.Users.GetBySubject(ctx, idTokenClaims.Subject)
		if err != nil {
			// User not found, create a new one.
			subject := idTokenClaims.Subject
			newUser := &models.User{
				Subject: &subject,
				Email:   idTokenClaims.Email,
				Name:    idTokenClaims.Name,
			}
			if err := deps.Users.Create(ctx, newUser); err != nil {
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
			user = newUser
		}

		// Create a new session
		tokenHash := auth.HashToken(tokens.AccessToken)
		newSession := &models.Session{
			UserID:    &user.ID,
			TokenHash: tokenHash,
			ExpiresAt: tokens.Expiry,
			IDToken:   rawIDToken,
		}
		if err := deps.Sessions.Create(ctx, newSession); err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		cookie := &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    tokens.AccessToken,
			Path:     "/",
			Expires:  tokens.Expiry,
			HttpOnly: true,
			Secure:   r.URL.Scheme == "https",
			SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(w, cookie)

		// Redirect to the frontend application, which is assumed to be at the root.
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// HandleLogout revokes the user's session.
func HandleLogout(deps *gridmiddleware.AuthnDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.GetUserFromContext(r.Context())
		if !ok {
			http.Error(w, "No active session", http.StatusUnauthorized)
			return
		}

		if err := deps.Sessions.Revoke(r.Context(), principal.SessionID); err != nil {
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
	Mode               string  `json:"mode"`                          // "external-idp" or "internal-idp"
	Issuer             string  `json:"issuer"`                        // OIDC issuer URL
	ClientID           *string `json:"client_id,omitempty"`           // Public client ID for device flow (Mode 1 only)
	Audience           *string `json:"audience,omitempty"`            // Expected aud claim in access tokens
	SupportsDeviceFlow bool    `json:"supports_device_flow"`          // Whether interactive device flow is supported
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
			response.ClientID = nil // No public client for device flow
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
