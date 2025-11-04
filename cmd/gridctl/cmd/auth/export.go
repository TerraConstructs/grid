package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	shellFormat string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export Terraform HTTP Backend authentication variables",
	Long: `Export authentication credentials as environment variables for Terraform HTTP Backend.

This command outputs shell commands to set TF_HTTP_PASSWORD and TF_HTTP_USERNAME
environment variables, which are required by Terraform/OpenTofu HTTP Backend.

Supported shells:
  - posix (bash, zsh, sh) - default
  - fish
  - powershell

Usage:
  # POSIX shells (bash/zsh/sh)
  eval $(gridctl auth export)

  # Fish shell
  eval (gridctl auth export --shell fish)

  # PowerShell
  gridctl auth export --shell powershell | Invoke-Expression

The credentials are loaded from your stored login session. If not logged in or
credentials are expired, you will be prompted to run 'gridctl auth login'.`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVar(&shellFormat, "shell", "", "Shell format: posix, fish, powershell (auto-detected if not specified)")
}

func runExport(cmd *cobra.Command, args []string) error {
	// Load credentials from the provider
	creds, err := clientProvider.Credentials()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w\n\nPlease run 'gridctl auth login' first", err)
	}

	// Check if token is expired
	if creds.IsExpired() {
		return fmt.Errorf("access token has expired\n\nPlease run 'gridctl auth login' to refresh your credentials")
	}

	// Auto-detect shell if not specified
	if shellFormat == "" {
		shellFormat = detectShell()
	}

	// Normalize shell format
	shellFormat = strings.ToLower(shellFormat)

	// Generate output based on shell format
	switch shellFormat {
	case "posix", "bash", "zsh", "sh":
		printPosixExport(creds.AccessToken)
	case "fish":
		printFishExport(creds.AccessToken)
	case "powershell", "pwsh", "ps1":
		printPowerShellExport(creds.AccessToken)
	default:
		return fmt.Errorf("unsupported shell format: %s\n\nSupported formats: posix, fish, powershell", shellFormat)
	}

	return nil
}

// detectShell attempts to detect the current shell from the SHELL environment variable
func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Default to POSIX if we can't detect
		return "posix"
	}

	// Extract the shell name from the path
	shellName := filepath.Base(shell)

	switch shellName {
	case "fish":
		return "fish"
	case "pwsh", "powershell":
		return "powershell"
	default:
		// Default to POSIX for bash, zsh, sh, and unknown shells
		return "posix"
	}
}

// printPosixExport outputs export commands for POSIX-compatible shells (bash, zsh, sh)
func printPosixExport(accessToken string) {
	// Only print instructions if stdout is a TTY (interactive mode, not being piped/eval'd)
	if isTerminal(os.Stdout) {
		fmt.Fprintln(os.Stderr, "# Run this command to configure your Terraform environment:")
		fmt.Fprintln(os.Stderr, "#   eval $(gridctl auth export)")
		fmt.Fprintln(os.Stderr, "")
	}
	// Print actual export commands to stdout for eval to process
	fmt.Printf("export TF_HTTP_PASSWORD=\"%s\"\n", accessToken)
	fmt.Println("export TF_HTTP_USERNAME=\"gridapi\"")
}

// printFishExport outputs set commands for Fish shell
func printFishExport(accessToken string) {
	// Only print instructions if stdout is a TTY (interactive mode, not being piped/eval'd)
	if isTerminal(os.Stdout) {
		fmt.Fprintln(os.Stderr, "# Run this command to configure your Terraform environment:")
		fmt.Fprintln(os.Stderr, "#   eval (gridctl auth export --shell fish)")
		fmt.Fprintln(os.Stderr, "")
	}
	// Print actual set commands to stdout for eval to process
	fmt.Printf("set -x TF_HTTP_PASSWORD \"%s\"\n", accessToken)
	fmt.Println("set -x TF_HTTP_USERNAME \"gridapi\"")
}

// printPowerShellExport outputs environment variable commands for PowerShell
func printPowerShellExport(accessToken string) {
	// Only print instructions if stdout is a TTY (interactive mode, not being piped/eval'd)
	if isTerminal(os.Stdout) {
		fmt.Fprintln(os.Stderr, "# Run this command to configure your Terraform environment:")
		fmt.Fprintln(os.Stderr, "#   gridctl auth export --shell powershell | Invoke-Expression")
		fmt.Fprintln(os.Stderr, "")
	}
	// Print actual PowerShell commands to stdout for Invoke-Expression to process
	fmt.Printf("$env:TF_HTTP_PASSWORD=\"%s\"\n", accessToken)
	fmt.Println("$env:TF_HTTP_USERNAME=\"gridapi\"")
}

// isTerminal checks if the given file is a terminal (TTY)
func isTerminal(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		return false
	}
	// Check if the file mode indicates it's a character device (terminal)
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
