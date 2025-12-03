package policy

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// PolicyCmd is the parent command for label policy operations.
var PolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage label validation policy",
	Long:  `Commands for inspecting and managing the Grid label validation policy.`,
}

func init() {
	PolicyCmd.AddCommand(getCmd)
	PolicyCmd.AddCommand(setCmd)
	PolicyCmd.AddCommand(complianceCmd)
}

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	cfg := config.MustFromContext(ctx)
	return cfg.ClientProvider.SDKClient(ctx)
}
