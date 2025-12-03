package auth

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/pkg/sdk"
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

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	cfg := config.MustFromContext(ctx)
	return cfg.ClientProvider.SDKClient(ctx)
}
