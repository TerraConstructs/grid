package deps

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

// DepsCmd is the parent command for dependency operations
var DepsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Manage state dependencies",
	Long:  `Commands for managing dependencies between Terraform states.`,
}

func init() {
	// Add subcommands (defined in separate files)
	DepsCmd.AddCommand(addCmd)
	DepsCmd.AddCommand(removeCmd)
	DepsCmd.AddCommand(listCmd)
	DepsCmd.AddCommand(searchCmd)
	DepsCmd.AddCommand(statusCmd)
	DepsCmd.AddCommand(topoCmd)
	DepsCmd.AddCommand(syncCmd)
}

// SetServerURL sets the server URL for all deps commands
func SetServerURL(url string) {
	ServerURL = url
}

// SetNonInteractive sets the non-interactive mode for all deps commands
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
