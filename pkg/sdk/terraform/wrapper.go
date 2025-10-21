package terraform

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/terraconstructs/grid/pkg/sdk"
)

// TF run options
type RunOptions struct {
	BinaryOverride string           // Override terraform/tofu binary (if empty, auto-detect)
	Args           []string         // Command-line arguments to pass to the binary
	Env            []string         // Additional environment variables
	OIDCEnabled    bool             // Whether OIDC is enabled on the server
	Credentials    *sdk.Credentials // Credentials for token injection
	ServerURL      string           // Grid API server URL (for context)
}

// Run executes a Terraform/Tofu binary with the given arguments.
// preserves the exact exit code from the subprocess, and runs in
// the current working directory. Ensures grid authentication and
// token injection.
//
// Parameters:
//   - ctx: Context for cancellation/timeout
//   - opts: RunOptions struct
//
// Returns:
//   - err: Error if the process couldn't be started or other system errors occurred
func Run(ctx context.Context, opts RunOptions) error {
	binary, err := FindTerraformBinary(opts.BinaryOverride)
	if err != nil {
		return fmt.Errorf("failed to find terraform binary: %w", err)
	}

	var authEnv []string
	// validate received credentials or attempt to load from env vars
	creds := opts.Credentials
	if opts.OIDCEnabled && (creds == nil || creds.IsExpired()) {
		hasEnvCreds, envCreds := sdk.CheckEnvCreds()
		if !hasEnvCreds {
			return fmt.Errorf("no valid credentials provided and no environment credentials found")
		}
		creds, err = sdk.LoginWithServiceAccount(ctx, envCreds.ClientID, envCreds.ClientSecret, opts.ServerURL)
		if err != nil {
			return fmt.Errorf("failed to authenticate with service account: %w", err)
		}
	}

	// Inject credentials if OIDC is enabled
	if opts.OIDCEnabled {
		if creds == nil {
			return fmt.Errorf("no credentials available for token injection")
		}
		// Inject access token via TF_HTTP_PASSWORD
		authEnv = []string{
			fmt.Sprintf("TF_HTTP_PASSWORD=%s", creds.AccessToken),
			// required by TF HTTP Backend, ignored by gridapi
			"TF_HTTP_USERNAME=gridapi",
		}
	}
	envs := append(opts.Env, authEnv...)

	// Create command with context for cancellation support
	cmd := exec.CommandContext(ctx, binary, opts.Args...)
	cmd.Env = append(os.Environ(), envs...)

	// Connect subprocess stdin/stdout/stderr to os.Stdin/os.Stdout/os.Stderr
	// This ensures transparent pass-through of all I/O
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run in current working directory (no directory change)
	// cmd.Dir is not set, so it defaults to the current directory

	// Start and wait for the process
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}
	return nil
}
