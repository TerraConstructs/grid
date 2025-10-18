package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/cmd/sa"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "gridapi",
	Short: "Grid API Server for Terraform state management",
	Long: `Grid API Server provides remote state management for Terraform and OpenTofu.
It exposes both Connect RPC and HTTP REST endpoints for state operations.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		return nil
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().String("db-url", "", "Database connection URL (env: DATABASE_URL)")
	rootCmd.PersistentFlags().String("server-addr", "", "Server bind address (env: SERVER_ADDR)")
	rootCmd.PersistentFlags().String("server-url", "", "Server base URL for backend config (env: SERVER_URL)")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging (env: DEBUG)")

	// Add subcommands
	rootCmd.AddCommand(sa.SaCmd)
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
