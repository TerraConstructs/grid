package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/deps"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/policy"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/state"
)

var (
	serverURL      string
	nonInteractive bool
)

var rootCmd = &cobra.Command{
	Use:   "gridctl",
	Short: "Grid CLI - Terraform state management client",
	Long: `gridctl is the command-line interface for Grid, a remote state management
system for Terraform and OpenTofu. Use it to create, list, and initialize states.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Check for GRID_NON_INTERACTIVE environment variable
		if os.Getenv("GRID_NON_INTERACTIVE") == "1" {
			nonInteractive = true
		}

		// Propagate flags to subcommands
		state.SetServerURL(serverURL)
		state.SetNonInteractive(nonInteractive)
		deps.SetServerURL(serverURL)
		deps.SetNonInteractive(nonInteractive)
		policy.SetServerURL(serverURL)
		policy.SetNonInteractive(nonInteractive)
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
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "Disable interactive prompts (also set via GRID_NON_INTERACTIVE=1)")
	rootCmd.AddCommand(state.StateCmd)
	rootCmd.AddCommand(deps.DepsCmd)
	rootCmd.AddCommand(policy.PolicyCmd)
}
