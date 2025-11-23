package sa

import "github.com/spf13/cobra"

var (
	rolesInput []string
)

// SaCmd is the parent command for service account operations
var SaCmd = &cobra.Command{
	Use:   "sa",
	Short: "Manage service accounts",
	Long:  `Commands for managing service accounts directly from the server.`,
}

func init() {
	SaCmd.AddCommand(listCmd)
	SaCmd.AddCommand(createCmd)
	createCmd.Flags().StringSliceVar(&rolesInput, "role", []string{}, "Role(s) to assign to the service account")
	SaCmd.AddCommand(assignCmd)
	assignCmd.Flags().StringSliceVar(&rolesInput, "role", []string{}, "Role(s) to assign to the service account")
	SaCmd.AddCommand(unassignCmd)
	unassignCmd.Flags().StringSliceVar(&rolesInput, "role", []string{}, "Role(s) to unassign from the service account")
}
