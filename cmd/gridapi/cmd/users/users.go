package users

import "github.com/spf13/cobra"

// UsersCmd is the parent command for user management operations
var UsersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage internal IdP users",
	Long:  `Commands for managing internal IdP users directly from the server.`,
}

func init() {
	createCmd.Flags().StringVar(&emailFlag, "email", "", "Email address of the user")
	createCmd.Flags().StringVar(&usernameFlag, "username", "", "Username/display name of the user")
	createCmd.Flags().StringVar(&passwordFlag, "password", "", "Password for the user (use --stdin to avoid shell history)")
	createCmd.Flags().StringSliceVar(&rolesInput, "role", []string{}, "Role(s) to assign to the user (required)")
	createCmd.Flags().BoolVar(&stdinFlag, "stdin", false, "Read password from stdin instead of --password flag")

	UsersCmd.AddCommand(createCmd)
}
