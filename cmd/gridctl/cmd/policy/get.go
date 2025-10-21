package policy

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the current label validation policy",
	RunE: func(cmd *cobra.Command, args []string) error {
		gridClient, err := sdkClient(cmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		policy, err := gridClient.GetLabelPolicy(ctx)
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				pterm.Info.Println("No label policy configured.")
				return nil
			}
			return fmt.Errorf("failed to get label policy: %w", err)
		}

		pterm.Printf("Version: %d\n", policy.Version)
		if !policy.CreatedAt.IsZero() {
			pterm.Printf("Created: %s\n", policy.CreatedAt.Format(time.RFC3339))
		}
		if !policy.UpdatedAt.IsZero() {
			pterm.Printf("Updated: %s\n", policy.UpdatedAt.Format(time.RFC3339))
		}
		pterm.Println()
		pterm.Println(policy.PolicyJSON)
		return nil
	},
}
