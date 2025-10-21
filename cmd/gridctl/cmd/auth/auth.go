package auth

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/client"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	// ServerURL is the Grid API server URL, set by the root command
	ServerURL string
	// NonInteractive controls whether interactive prompts are disabled
	NonInteractive bool

	clientProvider *client.Provider
)

// AuthCmd is the parent command for auth operations
var AuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  `Commands for managing authentication and login status.`,
}

func init() {
	AuthCmd.AddCommand(loginCmd)
	AuthCmd.AddCommand(logoutCmd)
	AuthCmd.AddCommand(statusCmd)
	AuthCmd.AddCommand(exportCmd)
}

// SetServerURL sets the server URL for all auth commands
func SetServerURL(url string) {
	ServerURL = url
}

// SetNonInteractive sets the non-interactive mode for all auth commands
func SetNonInteractive(value bool) {
	NonInteractive = value
}

// SetClientProvider injects the shared authenticated client provider.
func SetClientProvider(provider *client.Provider) {
	clientProvider = provider
}

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	if clientProvider == nil {
		return nil, fmt.Errorf("client provider not configured")
	}
	return clientProvider.SDKClient(ctx)
}
