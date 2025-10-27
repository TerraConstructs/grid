package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/auth"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/deps"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/policy"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/role"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/state"
	"github.com/terraconstructs/grid/cmd/gridctl/cmd/tf"
	internalclient "github.com/terraconstructs/grid/cmd/gridctl/internal/client"
)

var (
	serverURL      string
	nonInteractive bool
	bearerToken    string
	clientProvider *internalclient.Provider
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

		// Check for GRID_BEARER_TOKEN environment variable if --token not provided
		if bearerToken == "" {
			if envToken := os.Getenv("GRID_BEARER_TOKEN"); envToken != "" {
				bearerToken = envToken
			}
		}

		clientProvider = internalclient.NewProvider(serverURL)

		// Inject bearer token if provided (bypasses credential store)
		if bearerToken != "" {
			clientProvider.SetBearerToken(bearerToken)
		}

		// Propagate flags to subcommands
		state.SetServerURL(serverURL)
		state.SetNonInteractive(nonInteractive)
		state.SetClientProvider(clientProvider)
		deps.SetServerURL(serverURL)
		deps.SetNonInteractive(nonInteractive)
		deps.SetClientProvider(clientProvider)
		policy.SetServerURL(serverURL)
		policy.SetNonInteractive(nonInteractive)
		policy.SetClientProvider(clientProvider)
		auth.SetServerURL(serverURL)
		auth.SetNonInteractive(nonInteractive)
		auth.SetClientProvider(clientProvider)
		role.SetServerURL(serverURL)
		role.SetNonInteractive(nonInteractive)
		role.SetClientProvider(clientProvider)
		tf.SetServerURL(serverURL)
		tf.SetNonInteractive(nonInteractive)
		tf.SetClientProvider(clientProvider)
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
	rootCmd.PersistentFlags().StringVar(&bearerToken, "token", "", "Bearer token for authentication (bypasses credential store, also set via GRID_BEARER_TOKEN)")
	rootCmd.AddCommand(state.StateCmd)
	rootCmd.AddCommand(deps.DepsCmd)
	rootCmd.AddCommand(policy.PolicyCmd)
	rootCmd.AddCommand(auth.AuthCmd)
	rootCmd.AddCommand(role.RoleCmd)
	rootCmd.AddCommand(tf.TfCmd)
}
