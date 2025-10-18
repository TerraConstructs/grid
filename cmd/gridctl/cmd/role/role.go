package role

import (
	"github.com/spf13/cobra"
)

var (
	// ServerURL is the Grid API server URL, set by the root command
	ServerURL string
	// NonInteractive controls whether interactive prompts are disabled
	NonInteractive bool
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
