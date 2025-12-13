package state

import (
	"context"
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	restoreLogicID string
	restoreGUID    string
)

var restoreCmd = &cobra.Command{
	Use:   "restore [logic-id]",
	Short: "Restore a tombstoned state to active status",
	Long: `Restores a soft-deleted (tombstoned) state back to active status, making it
visible in default listings and accessible for Terraform operations.

The state to restore is resolved from the .grid context by default, or can be specified
explicitly using the optional positional argument or --logic-id/--guid flags.

State must be tombstoned and within the retention period. States past retention
cannot be restored and must be purged.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get optional positional argument (logic ID)
		positionalLogicID := ""
		if len(args) > 0 {
			positionalLogicID = args[0]
		}

		// Resolve state reference from positional arg, flags, or .grid context
		explicitRef := dirctx.StateRef{
			LogicID: restoreLogicID,
			GUID:    restoreGUID,
		}
		// Positional arg takes precedence over --logic-id flag
		if positionalLogicID != "" {
			explicitRef.LogicID = positionalLogicID
		}

		contextRef := dirctx.StateRef{}
		if gridCtx, err := dirctx.ReadGridContext(); err == nil && gridCtx != nil {
			contextRef.LogicID = gridCtx.StateLogicID
			contextRef.GUID = gridCtx.StateGUID
		}

		resolved, err := dirctx.ResolveStateRef(explicitRef, contextRef)
		if err != nil {
			return err
		}

		gridClient, err := sdkClient(cmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		// Call SDK RestoreState
		result, err := gridClient.RestoreState(ctx, sdk.StateReference{
			GUID:    resolved.GUID,
			LogicID: resolved.LogicID,
		})
		if err != nil {
			return fmt.Errorf("failed to restore state: %w", err)
		}

		// Display results
		pterm.Success.Printf("State restored successfully\n")
		pterm.Info.Printf("GUID:        %s\n", result.GUID)
		pterm.Info.Printf("Logic ID:    %s\n", result.LogicID)
		pterm.Info.Printf("Status:      %s\n", result.Status)
		pterm.Info.Printf("Restored at: %s\n", result.RestoredAt.Format(time.RFC3339))

		pterm.Success.Printf("\nState is now active and available for Terraform operations.\n")
		pterm.Info.Printf("Backend config:\n")
		pterm.Info.Printf("  Address:        %s\n", result.BackendConfig.Address)
		pterm.Info.Printf("  Lock Address:   %s\n", result.BackendConfig.LockAddress)
		pterm.Info.Printf("  Unlock Address: %s\n", result.BackendConfig.UnlockAddress)

		return nil
	},
}

func init() {
	restoreCmd.Flags().StringVar(&restoreLogicID, "logic-id", "", "State logic ID (overrides context)")
	restoreCmd.Flags().StringVar(&restoreGUID, "guid", "", "State GUID (overrides context)")
}
