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
	deleteLogicID string
	deleteGUID    string
)

var deleteCmd = &cobra.Command{
	Use:   "delete [logic-id]",
	Short: "Soft-delete (tombstone) a state",
	Long: `Marks a state as deleted (tombstoned), hiding it from default listings.
The state to delete is resolved from the .grid context by default, or can be specified
explicitly using the optional positional argument or --logic-id/--guid flags.

Soft-deleted states can be restored within the retention period. Data is preserved
until the state is purged (either automatically after retention expires, or manually
with --purge flag).

State must not be locked and must not have active dependents.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get optional positional argument (logic ID)
		positionalLogicID := ""
		if len(args) > 0 {
			positionalLogicID = args[0]
		}

		// Resolve state reference from positional arg, flags, or .grid context
		explicitRef := dirctx.StateRef{
			LogicID: deleteLogicID,
			GUID:    deleteGUID,
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

		// Call SDK TombstoneState
		result, err := gridClient.TombstoneState(ctx, sdk.StateReference{
			GUID:    resolved.GUID,
			LogicID: resolved.LogicID,
		})
		if err != nil {
			return fmt.Errorf("failed to tombstone state: %w", err)
		}

		// Display results
		pterm.Success.Printf("State deleted (tombstoned) successfully\n")
		pterm.Info.Printf("GUID:             %s\n", result.GUID)
		pterm.Info.Printf("Logic ID:         %s\n", result.LogicID)
		pterm.Info.Printf("Status:           %s\n", result.Status)
		pterm.Info.Printf("Tombstoned at:    %s\n", result.TombstonedAt.Format(time.RFC3339))
		pterm.Info.Printf("Tombstoned by:    %s\n", result.TombstonedBy)
		pterm.Info.Printf("Retention period: %d days\n", result.RetentionDays)
		pterm.Info.Printf("Purge eligible:   %s\n", result.PurgeEligibleAt.Format(time.RFC3339))

		pterm.Warning.Printf("\nState is soft-deleted and can be restored within %d days.\n", result.RetentionDays)
		pterm.Info.Printf("To restore: gridctl state restore --guid %s\n", result.GUID)
		pterm.Info.Printf("To purge permanently after retention: gridctl state delete --purge --guid %s\n", result.GUID)

		return nil
	},
}

func init() {
	deleteCmd.Flags().StringVar(&deleteLogicID, "logic-id", "", "State logic ID (overrides context)")
	deleteCmd.Flags().StringVar(&deleteGUID, "guid", "", "State GUID (overrides context)")
}
