package role

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

// SetServerURL sets the server URL for all role commands
func SetServerURL(url string) {
	ServerURL = url
}

// SetNonInteractive sets the non-interactive mode for all role commands
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
