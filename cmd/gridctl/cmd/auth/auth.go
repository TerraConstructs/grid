package auth

import (
	"github.com/spf13/cobra"
)

var (
	// ServerURL is the Grid API server URL, set by the root command
	ServerURL string
	// NonInteractive controls whether interactive prompts are disabled
	NonInteractive bool
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
}

// SetServerURL sets the server URL for all auth commands
func SetServerURL(url string) {
	ServerURL = url
}

// SetNonInteractive sets the non-interactive mode for all auth commands
func SetNonInteractive(value bool) {
	NonInteractive = value
}
