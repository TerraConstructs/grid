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
	deletePurge   bool
	deleteForce   bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete [logic-id]",
	Short: "Soft-delete (tombstone) or permanently purge a state",
	Long: `Marks a state as deleted (tombstoned), hiding it from default listings.
The state to delete is resolved from the .grid context by default, or can be specified
explicitly using the optional positional argument or --logic-id/--guid flags.

By default, this performs a soft-delete (tombstone). Soft-deleted states can be restored
within the retention period (default: 30 days). Data is preserved until the state is purged.

Use --purge flag to permanently delete a tombstoned state. Purge removes the state and all
associated data (outputs, schemas, dependencies) from the system. The logic_id becomes
available for reuse after purge.

Purge requires the state to be tombstoned first. By default, purge enforces the retention
period. Use --force to bypass the retention check and purge immediately.

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

		stateRef := sdk.StateReference{
			GUID:    resolved.GUID,
			LogicID: resolved.LogicID,
		}

		// Handle purge operation
		if deletePurge {
			result, err := gridClient.PurgeState(ctx, stateRef, deleteForce)
			if err != nil {
				return fmt.Errorf("failed to purge state: %w", err)
			}

			// Display purge results
			pterm.Success.Printf("State permanently purged\n")
			pterm.Info.Printf("GUID:       %s\n", result.GUID)
			pterm.Info.Printf("Logic ID:   %s\n", result.LogicID)
			pterm.Info.Printf("Purged at:  %s\n", result.PurgedAt.Format(time.RFC3339))

			if deleteForce {
				pterm.Warning.Printf("\nForce purge bypassed retention period check.\n")
			}
			pterm.Info.Printf("The logic_id '%s' is now available for reuse.\n", result.LogicID)

			return nil
		}

		// Handle tombstone (soft-delete) operation
		result, err := gridClient.TombstoneState(ctx, stateRef)
		if err != nil {
			return fmt.Errorf("failed to tombstone state: %w", err)
		}

		// Display tombstone results
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
		pterm.Info.Printf("To purge permanently: gridctl state delete --purge --guid %s\n", result.GUID)
		if result.RetentionDays > 0 {
			pterm.Info.Printf("Or wait until %s and purge will succeed without --force\n", result.PurgeEligibleAt.Format(time.RFC3339))
		}

		return nil
	},
}

func init() {
	deleteCmd.Flags().StringVar(&deleteLogicID, "logic-id", "", "State logic ID (overrides context)")
	deleteCmd.Flags().StringVar(&deleteGUID, "guid", "", "State GUID (overrides context)")
	deleteCmd.Flags().BoolVar(&deletePurge, "purge", false, "Permanently delete tombstoned state (cannot be undone)")
	deleteCmd.Flags().BoolVar(&deleteForce, "force", false, "Force purge within retention period (requires --purge)")
}
