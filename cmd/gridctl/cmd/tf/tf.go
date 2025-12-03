package tf

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
	"github.com/terraconstructs/grid/pkg/sdk/terraform"
)

var (
	// Command flags
	tfBin      string
	verbose    bool
	getLogicID string
	getGUID    string
)

// TfCmd is the parent command for terraform wrapper operations
var TfCmd = &cobra.Command{
	Use:   "tf [flags] -- <terraform args>",
	Short: "Run Terraform with Grid authentication",
	Long: `Wraps Terraform execution with automatic Grid authentication.

This command:
- Reads .grid context for backend configuration
- Injects Grid API credentials as environment variables
- Runs terraform with your provided arguments
- Preserves all I/O and exit codes

Example:
  gridctl tf -- init
  gridctl tf -- plan
  gridctl tf --tf-bin=tofu -- apply

The '--' separator is required to distinguish gridctl flags from terraform arguments.`,
	RunE: runTfCommand,
	// Allow all unknown flags to be passed through to terraform
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true,
	},
	// Disable flag parsing after "--"
	DisableFlagParsing: false,
}

func init() {
	TfCmd.Flags().StringVar(&getLogicID, "logic-id", "", "State logic ID (overrides positional arg and context)")
	TfCmd.Flags().StringVar(&getGUID, "guid", "", "State GUID (overrides positional arg and context)")
	TfCmd.Flags().StringVar(&tfBin, "tf-bin", "", "Path to terraform/tofu binary (defaults to TERRAFORM_BINARY_NAME env var or 'terraform')")
	TfCmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output with redacted credentials")
}

func runTfCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := config.MustFromContext(ctx)
	// Build explicit reference from flags/args
	explicitRef := dirctx.StateRef{}
	if getLogicID != "" {
		explicitRef.LogicID = getLogicID
	} else if getGUID != "" {
		explicitRef.GUID = getGUID
	}

	// Try to read .grid context
	contextRef := dirctx.StateRef{}
	gridCtx, err := dirctx.ReadGridContext()
	if err != nil {
		pterm.Warning.Printf("Warning: .grid file corrupted or invalid, ignoring: %v\n", err)
	} else if gridCtx != nil {
		contextRef.LogicID = gridCtx.StateLogicID
		contextRef.GUID = gridCtx.StateGUID
	}

	// Resolve final state reference
	stateRef, err := dirctx.ResolveStateRef(explicitRef, contextRef)
	if err != nil {
		return err
	}

	if gridCtx == nil {
		return fmt.Errorf(".grid context file not found - run 'gridctl state init' first")
	}

	// Check if backend.tf exists, print helpful hint if missing but
	// inject backend env vars from state config instead.
	var backendEnv = []string{}
	if _, err := os.Stat("backend.tf"); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: backend.tf not found. Consider running 'gridctl state init' to generate it.\n")
		// fetch state info for backend URL configuration

		gridClient, err := sdkClient(ctx)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Lookup state by logic ID
		state, err := gridClient.GetState(ctx, sdk.StateReference{
			LogicID: stateRef.LogicID,
			GUID:    stateRef.GUID,
		})
		if err != nil {
			return fmt.Errorf("failed to get state config: %w", err)
		}
		backendEnv = []string{
			// not allowed see https://github.com/opentofu/opentofu/issues/2058#issuecomment-2403376553
			// "TF_CLI_ARGS=-backend='http'",
			fmt.Sprintf("TF_HTTP_ADDRESS=%s", state.BackendConfig.Address),
			fmt.Sprintf("TF_HTTP_LOCK_ADDRESS=%s", state.BackendConfig.LockAddress),
			fmt.Sprintf("TF_HTTP_UNLOCK_ADDRESS=%s", state.BackendConfig.UnlockAddress),
		}
		// HACK: write temp "backend.tf" file with minimal tf > "http" backend config
		if err := os.WriteFile("backend.tf", []byte("terraform {\n  backend \"http\" {}\n}"), 0644); err != nil {
			return fmt.Errorf("failed to write temporary backend.tf: %w", err)
		}
		defer func() { _ = os.Remove("backend.tf") }()
	}

	// ignore error, wrapper will handle missing creds
	var creds *sdk.Credentials
	oidcEnabled, err := cfg.ClientProvider.IsOIDCEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to determine if OIDC is enabled: %w", err)
	}
	if oidcEnabled {
		creds, _ = cfg.ClientProvider.Credentials()
	}
	err = terraform.Run(ctx, terraform.RunOptions{
		ServerURL:      cfg.ServerURL,
		BinaryOverride: tfBin,
		Env:            backendEnv,
		OIDCEnabled:    oidcEnabled,
		Credentials:    creds,
		Args:           args,
	})
	if err != nil {
		return fmt.Errorf("failed to run terraform command: %w", err)
	}
	return nil
}

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	cfg := config.MustFromContext(ctx)
	return cfg.ClientProvider.SDKClient(ctx)
}
