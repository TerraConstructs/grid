package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/terraconstructs/grid/cmd/gridctl/internal/auth"
	"golang.org/x/oauth2"
)

// NewAuthenticatedClient creates a new http.Client that authenticates with the Grid server.
func NewAuthenticatedClient(serverURL string) (*http.Client, error) {
	store, err := auth.NewFileStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create credential store: %w", err)
	}

	creds, err := store.LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("not logged in")
	}

	token := &oauth2.Token{
		AccessToken:  creds.AccessToken,
		TokenType:    creds.TokenType,
		RefreshToken: creds.RefreshToken,
		Expiry:       creds.ExpiresAt,
	}

	source := oauth2.StaticTokenSource(token)
	return oauth2.NewClient(context.Background(), source), nil
}
