package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client/rp"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"
)

// RelyingParty handles OIDC authentication against an external IdP by wrapping
// the zitadel/oidc RelyingParty implementation.
type RelyingParty struct {
	rp rp.RelyingParty
}

// RP returns the underlying zitadel/oidc RelyingParty interface.
func (r *RelyingParty) RP() rp.RelyingParty {
	return r.rp
}

// NewRelyingParty creates a new RelyingParty for external IdP authentication.
func NewRelyingParty(ctx context.Context, cfg *config.ExternalIdPConfig) (*RelyingParty, error) {
	// The hash and crypto keys should be sourced from a secure configuration in production.
	// For local development, we generate random keys on startup.
	hashKey, err := generateRandomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cookie hash key: %w", err)
	}
	cryptoKey, err := generateRandomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cookie crypto key: %w", err)
	}

	cookieHandler := httphelper.NewCookieHandler(hashKey, cryptoKey, httphelper.WithUnsecure()) // Use WithUnsecure for local dev over HTTP

	// Custom unauthorized handler to provide visibility into OIDC callback errors.
	// This is critical for debugging issues like missing id_token, state mismatches, or PKCE failures.
	unauthorizedHandler := func(w http.ResponseWriter, r *http.Request, desc string, state string) {
		// Log structured error with context
		cookieNames := make([]string, 0, len(r.Cookies()))
		for _, c := range r.Cookies() {
			cookieNames = append(cookieNames, c.Name)
		}

		log.Printf("OIDC authentication failed: %s (state=%s, cookies=%v, path=%s)",
			desc, state, cookieNames, r.URL.Path)

		http.Error(w, "Authentication failed", http.StatusUnauthorized)
	}

	options := []rp.Option{
		rp.WithCookieHandler(cookieHandler),
		rp.WithVerifierOpts(rp.WithIssuedAtMaxAge(10 * time.Second)),
		rp.WithPKCE(cookieHandler), // Use the same cookie handler for PKCE
		rp.WithUnauthorizedHandler(unauthorizedHandler),
	}

	// Use configured scopes (defaults to [openid, profile, email] set in config.go)
	// Group memberships are obtained via JWT claim mapper, not via scope request
	relyingParty, err := rp.NewRelyingPartyOIDC(ctx, cfg.Issuer, cfg.ClientID, cfg.ClientSecret, cfg.RedirectURI,
		cfg.Scopes, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC relying party: %w", err)
	}

	return &RelyingParty{rp: relyingParty}, nil
}

// generateRandomBytes creates a slice of random bytes of a specified size.
func generateRandomBytes(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}

// GenerateNonce generates a random nonce string.
func GenerateNonce() (string, error) {
	b, err := generateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SetRedirectURICookie stores the redirect URI in a temporary cookie for the SSO flow.
// This cookie is short-lived (10 minutes) and used to remember where to redirect after OAuth callback.
// Related: Beads issue grid-202d (SSO callback redirect fix)
func SetRedirectURICookie(w http.ResponseWriter, r *http.Request, redirectURI string) {
	cookie := &http.Cookie{
		Name:     "grid.redirect_uri",
		Value:    redirectURI,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   r.URL.Scheme == "https", // Automatically use Secure only over HTTPS
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// GetRedirectURICookie retrieves and clears the redirect URI cookie.
// Returns empty string if cookie not found or expired.
// Related: Beads issue grid-202d (SSO callback redirect fix)
func GetRedirectURICookie(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("grid.redirect_uri")
	if err != nil {
		return ""
	}

	// Clear the cookie after reading
	clearCookie := &http.Cookie{
		Name:     "grid.redirect_uri",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   r.URL.Scheme == "https", // Match the original cookie's Secure flag
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, clearCookie)

	return cookie.Value
}
