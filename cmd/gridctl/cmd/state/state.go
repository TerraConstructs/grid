package state

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/pkg/sdk"
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

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	cfg := config.MustFromContext(ctx)
	return cfg.ClientProvider.SDKClient(ctx)
}
