package state

import (
	"github.com/spf13/cobra"
)

var (
	// ServerURL is the Grid API server URL, set by the root command
	ServerURL string
	// NonInteractive controls whether interactive prompts are disabled
	NonInteractive bool
)

// StateCmd is the parent command for state operations
var StateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage Terraform states",
	Long:  `Commands for creating, listing, and initializing Terraform remote states.`,
}

func init() {
	// Add subcommands (defined in separate files)
	StateCmd.AddCommand(createCmd)
	StateCmd.AddCommand(listCmd)
	StateCmd.AddCommand(getCmd)
	StateCmd.AddCommand(setCmd)
	StateCmd.AddCommand(initCmd)
}

// SetServerURL sets the server URL for all state commands
func SetServerURL(url string) {
	ServerURL = url
}

// SetNonInteractive sets the non-interactive mode for all state commands
func SetNonInteractive(value bool) {
	NonInteractive = value
}
