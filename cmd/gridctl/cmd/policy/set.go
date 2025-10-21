package policy

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	setFile string
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Apply a new label validation policy",
	RunE: func(cmd *cobra.Command, args []string) error {
		if setFile == "" {
			return fmt.Errorf("--file is required")
		}

		contents, err := os.ReadFile(setFile)
		if err != nil {
			return fmt.Errorf("failed to read policy file: %w", err)
		}

		gridClient, err := sdkClient(cmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		policy, err := gridClient.SetLabelPolicy(ctx, contents)
		if err != nil {
			return fmt.Errorf("failed to set label policy: %w", err)
		}

		pterm.Success.Printf("Policy updated to version %d\n", policy.Version)
		if !policy.UpdatedAt.IsZero() {
			pterm.Info.Printf("Updated at %s\n", policy.UpdatedAt.Format(time.RFC3339))
		}
		return nil
	},
}

func init() {
	setCmd.Flags().StringVar(&setFile, "file", "", "Path to JSON policy definition")
}
