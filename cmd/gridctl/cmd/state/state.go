package state

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

// StateCmd is the parent command for state operations
var StateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage Terraform states",
	Long:  `Commands for creating, listing, and initializing Terraform remote states.`,
}

func init() {
	// Add subcommands (defined in separate files)
	StateCmd.AddCommand(createCmd)
	StateCmd.AddCommand(listCmd)
	StateCmd.AddCommand(getCmd)
	StateCmd.AddCommand(setCmd)
	StateCmd.AddCommand(initCmd)
	StateCmd.AddCommand(setOutputSchemaCmd)
	StateCmd.AddCommand(getOutputSchemaCmd)
}

// SetServerURL sets the server URL for all state commands
func SetServerURL(url string) {
	ServerURL = url
}

// SetNonInteractive sets the non-interactive mode for all state commands
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
