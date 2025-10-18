package sa

import (
	"github.com/spf13/cobra"
)

// SaCmd is the parent command for service account operations
var SaCmd = &cobra.Command{
	Use:   "sa",
	Short: "Manage service accounts",
	Long:  `Commands for managing service accounts directly from the server.`,
}
