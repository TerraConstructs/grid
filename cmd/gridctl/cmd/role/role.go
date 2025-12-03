package role

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// RoleCmd is the parent command for role operations
var RoleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage roles and permissions",
	Long:  `Commands for managing roles, permissions, and assignments.`,
}

func init() {
	RoleCmd.AddCommand(inspectCmd)
	RoleCmd.AddCommand(assignGroupCmd)
	RoleCmd.AddCommand(removeGroupCmd)
	RoleCmd.AddCommand(listGroupsCmd)
	RoleCmd.AddCommand(exportCmd)
	RoleCmd.AddCommand(importCmd)
}

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	cfg := config.MustFromContext(ctx)
	return cfg.ClientProvider.SDKClient(ctx)
}
