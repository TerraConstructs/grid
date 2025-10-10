package policy

import (
	"github.com/spf13/cobra"
)

var (
	serverURL      string
	nonInteractive bool
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

// SetServerURL sets the Grid API server URL for policy commands.
func SetServerURL(url string) {
	serverURL = url
}

// SetNonInteractive controls interactive prompts (reserved for future use).
func SetNonInteractive(value bool) {
	nonInteractive = value
}
