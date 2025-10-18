package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"
)

// RelyingParty handles OIDC authentication against an external IdP by wrapping
// the zitadel/oidc RelyingParty implementation.
type RelyingParty struct {
	rp rp.RelyingParty
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

	options := []rp.Option{
		rp.WithCookieHandler(cookieHandler),
		rp.WithVerifierOpts(rp.WithIssuedAtMaxAge(10 * time.Second)),
		rp.WithPKCE(cookieHandler), // Use the same cookie handler for PKCE
	}

	relyingParty, err := rp.NewRelyingPartyOIDC(ctx, cfg.Issuer, cfg.ClientID, cfg.ClientSecret, cfg.RedirectURI,
		[]string{oidc.ScopeOpenID, "profile", "email", "groups"}, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC relying party: %w", err)
	}

	return &RelyingParty{rp: relyingParty}, nil
}

// AuthCodeURL returns the URL for the authorization endpoint.
func (r *RelyingParty) AuthCodeURL(state string) string {
	return rp.AuthURL(state, r.rp)
}

// Exchange exchanges an authorization code for an OIDC token and verified claims.
func (r *RelyingParty) Exchange(ctx context.Context, code string) (*oidc.Tokens[*oidc.IDTokenClaims], error) {
	return rp.CodeExchange[*oidc.IDTokenClaims](ctx, code, r.rp)
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

// SetStateCookie sets the state nonce in a cookie.
func SetStateCookie(w http.ResponseWriter, state string) {
	cookie := &http.Cookie{
		Name:     "grid.state",
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   true, // Set to false in local dev if not using HTTPS
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// VerifyStateCookie verifies the state nonce from the cookie.
func VerifyStateCookie(r *http.Request, state string) error {
	cookie, err := r.Cookie("grid.state")
	if err != nil {
		return fmt.Errorf("state cookie not found")
	}
	if cookie.Value != state {
		return fmt.Errorf("invalid state")
	}
	return nil
}
