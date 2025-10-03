package deps

import (
	"github.com/spf13/cobra"
)

var (
	// ServerURL is the Grid API server URL, set by the root command
	ServerURL string
)

// DepsCmd is the parent command for dependency operations
var DepsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Manage state dependencies",
	Long:  `Commands for managing dependencies between Terraform states.`,
}

func init() {
	// Add subcommands (defined in separate files)
	DepsCmd.AddCommand(addCmd)
	DepsCmd.AddCommand(removeCmd)
	DepsCmd.AddCommand(listCmd)
	DepsCmd.AddCommand(searchCmd)
	DepsCmd.AddCommand(statusCmd)
	DepsCmd.AddCommand(topoCmd)
	DepsCmd.AddCommand(syncCmd)
}

// SetServerURL sets the server URL for all deps commands
func SetServerURL(url string) {
	ServerURL = url
}
