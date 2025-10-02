package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/state"
)

var (
	serverURL string
)

var rootCmd = &cobra.Command{
	Use:   "gridctl",
	Short: "Grid CLI - Terraform state management client",
	Long: `gridctl is the command-line interface for Grid, a remote state management
system for Terraform and OpenTofu. Use it to create, list, and initialize states.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Propagate serverURL to state subcommands
		state.SetServerURL(serverURL)
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "Grid API server URL")
	rootCmd.AddCommand(state.StateCmd)
}
