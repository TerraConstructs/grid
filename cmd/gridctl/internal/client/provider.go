package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/auth"
	"github.com/terraconstructs/grid/pkg/sdk"
	"golang.org/x/oauth2"
)

// Provider yields authenticated HTTP and SDK clients backed by the credential store.
type Provider struct {
	serverURL   string
	bearerToken string // ephemeral token that bypasses credential store (for testing)

	httpOnce sync.Once
	httpCli  *http.Client
	httpErr  error

	credentialsOnce sync.Once
	credentials     *sdk.Credentials
	credentialsErr  error

	sdkOnce   sync.Once
	sdkClient *sdk.Client
	sdkErr    error

	oidcOnce     sync.Once
	oidcEnabled  bool
	oidcErr      error
	oidcWarnOnce sync.Once
}

// NewProvider constructs a new Provider bound to the given server URL.
func NewProvider(serverURL string) *Provider {
	return &Provider{serverURL: serverURL}
}

// SetBearerToken injects an ephemeral bearer token for testing (bypasses credential store).
func (p *Provider) SetBearerToken(token string) {
	p.bearerToken = token
}

func (p *Provider) IsOIDCEnabled(ctx context.Context) (bool, error) {
	return p.oidcStatus(ctx)
}

func (p *Provider) Credentials() (*sdk.Credentials, error) {
	p.credentialsOnce.Do(func() {
		store, err := auth.NewFileStore()
		if err != nil {
			p.credentialsErr = err
			return
		}

		creds, err := store.LoadCredentials()
		if err != nil {
			p.credentialsErr = err
			return
		}

		p.credentials = creds
	})
	if p.credentialsErr != nil {
		return nil, p.credentialsErr
	}

	return p.credentials, nil
}

// HTTPClient returns an http.Client configured for the server's auth mode.
// When OIDC is disabled, http.DefaultClient is returned alongside a warning.
func (p *Provider) HTTPClient(ctx context.Context) (*http.Client, error) {
	p.httpOnce.Do(func() {
		// Priority 1: Ephemeral bearer token (for testing/CI)
		if p.bearerToken != "" {
			token := &oauth2.Token{
				AccessToken: p.bearerToken,
				TokenType:   "Bearer",
			}
			source := oauth2.StaticTokenSource(token)
			p.httpCli = oauth2.NewClient(context.Background(), source)
			return
		}

		// Priority 2: Existing credential store flow
		ctx, cancel := ensureTimeout(ctx, 5*time.Second)
		defer cancel()

		enabled, err := p.oidcStatus(ctx)
		if err != nil {
			p.httpErr = fmt.Errorf("unable to determine authentication mode: %w", err)
			return
		}

		if !enabled {
			p.oidcWarnOnce.Do(func() {
				pterm.Warning.Printf("OIDC authentication disabled for %s; proceeding without credentials.\n", p.serverURL)
			})
			p.httpCli = http.DefaultClient
			return
		}
		creds, err := p.Credentials()
		if err != nil {
			p.httpErr = err
			return
		}
		if creds == nil {
			p.httpErr = errors.New("no credentials found; please run `gridctl auth login`")
			return
		}

		if creds.IsExpired() {
			p.httpErr = errors.New("access token expired; please run `gridctl auth login`")
			return
		}

		token := &oauth2.Token{
			AccessToken:  creds.AccessToken,
			TokenType:    creds.TokenType,
			RefreshToken: creds.RefreshToken,
			Expiry:       creds.ExpiresAt,
		}

		source := oauth2.StaticTokenSource(token)
		p.httpCli = oauth2.NewClient(context.Background(), source)
	})

	if p.httpErr != nil {
		return nil, p.httpErr
	}

	return p.httpCli, nil
}

// SDKClient returns an authenticated SDK client backed by HTTPClient.
func (p *Provider) SDKClient(ctx context.Context) (*sdk.Client, error) {
	p.sdkOnce.Do(func() {
		httpClient, err := p.HTTPClient(ctx)
		if err != nil {
			p.sdkErr = err
			return
		}

		p.sdkClient = sdk.NewClient(p.serverURL, sdk.WithHTTPClient(httpClient))
	})

	if p.sdkErr != nil {
		return nil, p.sdkErr
	}

	return p.sdkClient, nil
}

// oidcStatus probes the /health endpoint once to determine if OIDC is enabled.
func (p *Provider) oidcStatus(ctx context.Context) (bool, error) {
	p.oidcOnce.Do(func() {
		ctx, cancel := ensureTimeout(ctx, 3*time.Second)
		defer cancel()

		healthURL, err := url.JoinPath(p.serverURL, "/health")
		if err != nil {
			p.oidcErr = fmt.Errorf("invalid server URL: %w", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			p.oidcErr = fmt.Errorf("failed to build health request: %w", err)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			p.oidcErr = fmt.Errorf("health request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			p.oidcErr = fmt.Errorf("health endpoint returned %s", resp.Status)
			return
		}

		var payload struct {
			OIDCEnabled bool `json:"oidc_enabled"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			p.oidcErr = fmt.Errorf("failed to decode health response: %w", err)
			return
		}

		p.oidcEnabled = payload.OIDCEnabled
	})

	return p.oidcEnabled, p.oidcErr
}

func ensureTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	return ctxWithTimeout, cancel
}
