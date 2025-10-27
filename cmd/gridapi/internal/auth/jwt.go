package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/xenitab/go-oidc-middleware/oidctoken"
	"github.com/xenitab/go-oidc-middleware/options"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

const SessionCookieName = "grid.session"

type claimsContextKey struct{}

var defaultClaimsContextKey = claimsContextKey{}

type tokenStringContextKey struct{}

var defaultTokenStringContextKey = tokenStringContextKey{}

type tokenHashContextKey struct{}

var defaultTokenHashContextKey = tokenHashContextKey{}

// Skipper defines a function to skip authentication for matching requests.
type Skipper func(*http.Request) bool

// ErrorResponder writes authentication failures to the response writer.
type ErrorResponder func(http.ResponseWriter, *http.Request, error)

type verifierOptions struct {
	skipper        Skipper
	errorResponder ErrorResponder
	tokenStrings   [][]options.TokenStringOption
}

// VerifierOption customises the behaviour of the OIDC verifier middleware.
type VerifierOption func(*verifierOptions)

// WithSkipper overrides the default skipper used by the verifier.
func WithSkipper(skipper Skipper) VerifierOption {
	return func(o *verifierOptions) {
		if skipper != nil {
			o.skipper = skipper
		}
	}
}

// WithErrorResponder overrides the default error responder used by the verifier.
func WithErrorResponder(responder ErrorResponder) VerifierOption {
	return func(o *verifierOptions) {
		if responder != nil {
			o.errorResponder = responder
		}
	}
}

// WithTokenStringOptions appends a token extraction strategy used by the verifier.
// The provided setters mirror go-oidc-middleware's token string configuration helpers.
func WithTokenStringOptions(setters ...options.TokenStringOption) VerifierOption {
	return func(o *verifierOptions) {
		o.tokenStrings = append(o.tokenStrings, setters)
	}
}

// WithTokenString configures an alternate header and prefix that should be treated as a bearer token.
func WithTokenString(header, prefix string) VerifierOption {
	tokenPrefix := prefix
	if tokenPrefix == "" {
		tokenPrefix = "Bearer "
	}
	return WithTokenStringOptions(
		options.WithTokenStringHeaderName(header),
		options.WithTokenStringTokenPrefix(tokenPrefix),
	)
}

// NewVerifier constructs a Chi-compatible middleware that validates JWTs using go-oidc-middleware.
func NewVerifier(cfg *config.Config, opts ...VerifierOption) (func(http.Handler) http.Handler, error) {
	var issuer, clientID string
	var isInternalProvider bool

	// Mode detection
	if cfg.OIDC.ExternalIdP != nil {
		// Mode 1: External IdP Only
		// In Mode 1, Grid is a Resource Server validating tokens from an external IdP.
		// Clients must request tokens with audience=ExternalIdP.ClientID (the resource server identifier).
		// Grid validates that tokens include this audience in the aud claim.
		issuer = cfg.OIDC.ExternalIdP.Issuer
		clientID = cfg.OIDC.ExternalIdP.ClientID // This is the resource server identifier (e.g., "grid-api")
		isInternalProvider = false
	} else if cfg.OIDC.Issuer != "" {
		// Mode 2: Internal IdP Only
		issuer = cfg.OIDC.Issuer
		clientID = cfg.OIDC.ClientID
		isInternalProvider = true
	} else {
		// Auth disabled
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}, nil
	}

	if issuer == "" {
		return nil, errors.New("oidc issuer is required")
	}
	if clientID == "" {
		return nil, errors.New("oidc client id is required")
	}

	oidcOpts := []options.Option{
		options.WithIssuer(issuer),
		options.WithRequiredAudience(clientID),
	}

	// CRITICAL: For internal provider, use lazy load to avoid race condition during server startup
	// The OIDC endpoints aren't available yet when the verifier is being initialized
	if isInternalProvider {
		oidcOpts = append(oidcOpts, options.WithLazyLoadJwks(true))
	}

	tokenHandler, err := oidctoken.New[map[string]any](nil, oidcOpts...)
	if err != nil {
		return nil, fmt.Errorf("initialise oidc token handler: %w", err)
	}

	vOpts := verifierOptions{
		skipper:        defaultSkipper,
		errorResponder: defaultErrorResponder,
	}
	for _, opt := range opts {
		opt(&vOpts)
	}

	tokenStrings := make([][]options.TokenStringOption, 0, len(vOpts.tokenStrings)+1)
	tokenStrings = append(tokenStrings, vOpts.tokenStrings...)
	tokenStrings = append(tokenStrings, []options.TokenStringOption{}) // Default: Authorization header.

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if vOpts.skipper != nil && vOpts.skipper(r) {
				next.ServeHTTP(w, r)
				return
			}

			token, err := oidctoken.GetTokenString(r.Header.Get, tokenStrings)
			if err != nil || token == "" {
				vOpts.errorResponder(w, r, fmt.Errorf("unable to extract bearer token: %w", err))
				return
			}

			trimmedToken := strings.TrimSpace(token)

			claims, err := tokenHandler.ParseToken(r.Context(), trimmedToken)
			if err != nil {
				vOpts.errorResponder(w, r, fmt.Errorf("invalid token: %w", err))
				return
			}

			// Validate JTI claim exists
			jti, ok := claims["jti"].(string)
			if !ok || jti == "" {
				vOpts.errorResponder(w, r, fmt.Errorf("token missing jti claim"))
				return
			}

			ctx := context.WithValue(r.Context(), defaultClaimsContextKey, claims)
			ctx = context.WithValue(ctx, defaultTokenStringContextKey, trimmedToken)
			ctx = context.WithValue(ctx, defaultTokenHashContextKey, HashToken(jti)) // Hash the JTI for session lookup

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// ClaimsFromContext returns the JWT claims stored on the request context.
func ClaimsFromContext(ctx context.Context) (map[string]any, bool) {
	claims, ok := ctx.Value(defaultClaimsContextKey).(map[string]any)
	return claims, ok
}

// TokenStringFromContext returns the raw bearer token extracted during verification.
func TokenStringFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(defaultTokenStringContextKey).(string)
	return token, ok
}

// TokenHashFromContext returns the SHA256 hash of the bearer token extracted during verification.
func TokenHashFromContext(ctx context.Context) (string, bool) {
	hash, ok := ctx.Value(defaultTokenHashContextKey).(string)
	return hash, ok
}

// JTIFromClaims extracts the JWT ID claim from the claims map.
// This is used for revocation checking in the authentication middleware.
func JTIFromClaims(claims map[string]any) (string, bool) {
	jti, ok := claims["jti"].(string)
	return jti, ok && jti != ""
}

func defaultSkipper(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.Method == http.MethodOptions {
		return true
	}

	path := r.URL.Path

	// Public prefixes that should not be subjected to bearer token authentication.
	publicPrefixes := []string{
		"/health",
		"/auth/",
		"/.well-known/",
		"/device_authorization",
		"/token",
		"/jwks",
		"/keys", // JWKS endpoint for JWT signature verification
		"/authorization",
		"/oauth/",
	}

	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

func defaultErrorResponder(w http.ResponseWriter, _ *http.Request, err error) {
	_ = err // TODO: connect audit logging (FR-098/FR-099).
	http.Error(w, "unauthenticated", http.StatusUnauthorized)
}

// HashToken creates a SHA256 hash of a token string.
func HashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}
