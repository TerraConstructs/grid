package dep

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// DepCmd is the parent command for dependency operations
var DepCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage state dependencies",
	Long:  `Commands for managing dependencies between Terraform states.`,
}

func init() {
	// Add subcommands (defined in separate files)
	DepCmd.AddCommand(addCmd)
	DepCmd.AddCommand(removeCmd)
	DepCmd.AddCommand(listCmd)
	DepCmd.AddCommand(searchCmd)
	DepCmd.AddCommand(statusCmd)
	DepCmd.AddCommand(topoCmd)
	DepCmd.AddCommand(syncCmd)
}

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	cfg := config.MustFromContext(ctx)
	return cfg.ClientProvider.SDKClient(ctx)
}
