package deps

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/pkg/sdk"
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

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	cfg := config.MustFromContext(ctx)
	return cfg.ClientProvider.SDKClient(ctx)
}
