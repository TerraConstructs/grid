package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/terraconstructs/grid/cmd/gridapi/cmd/iam"
	"github.com/terraconstructs/grid/cmd/gridapi/cmd/sa"
	"github.com/terraconstructs/grid/cmd/gridapi/cmd/users"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

var cfg *config.Config

// Version information (set by main package via SetVersion)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

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
	// Initialize Viper before binding flags
	cobra.OnInitialize(initConfig)

	// Optional --config flag to override default search paths
	rootCmd.PersistentFlags().String("config", "", "Config file path (YAML/JSON/TOML - overrides default search)")
	viper.BindPFlag("config_file", rootCmd.PersistentFlags().Lookup("config"))

	// Core configuration flags
	rootCmd.PersistentFlags().String("db-url", "", "Database connection URL (GRID_DATABASE_URL)")
	rootCmd.PersistentFlags().String("server-addr", "", "Server bind address (GRID_SERVER_ADDR)")
	rootCmd.PersistentFlags().String("server-url", "", "Server base URL for backend config (GRID_SERVER_URL)")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging (GRID_DEBUG)")
	rootCmd.PersistentFlags().Int("max-db-connections", 0, "Max DB connections (GRID_MAX_DB_CONNECTIONS)")

	// Bind flags to Viper keys
	viper.BindPFlag("database_url", rootCmd.PersistentFlags().Lookup("db-url"))
	viper.BindPFlag("server_addr", rootCmd.PersistentFlags().Lookup("server-addr"))
	viper.BindPFlag("server_url", rootCmd.PersistentFlags().Lookup("server-url"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("max_db_connections", rootCmd.PersistentFlags().Lookup("max-db-connections"))

	// Add subcommands
	rootCmd.AddCommand(sa.SaCmd)
	rootCmd.AddCommand(iam.IamCmd)
	rootCmd.AddCommand(users.UsersCmd)
	rootCmd.AddCommand(versionCmd)
}

// initConfig initializes Viper configuration from config files and environment
func initConfig() {
	// If --config flag provided, use that file exclusively
	if cfgFile := viper.GetString("config_file"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Auto-discover config in default locations
		viper.SetConfigName("gridapi")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")           // Current directory
		viper.AddConfigPath("$HOME/.grid") // User home
		viper.AddConfigPath("/etc/grid")   // System-wide
	}

	// Read config file (silent if not found - not required)
	_ = viper.ReadInConfig()
}

// GetConfig returns the loaded configuration
// This should be called after the root command's PersistentPreRunE has executed
func GetConfig() *config.Config {
	return cfg
}

// SetVersion sets version information from the main package
func SetVersion(v, c, d, b string) {
	version = v
	commit = c
	date = d
	builtBy = b
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gridapi version %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built: %s\n", date)
		fmt.Printf("  by: %s\n", builtBy)
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
