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
	renameLogicID string
	renameGUID    string
)

var renameCmd = &cobra.Command{
	Use:   "rename <new-logic-id>",
	Short: "Rename a state's logic ID",
	Long: `Renames a state's logic ID while preserving its GUID and backend configuration.
The state to rename is resolved from the .grid context by default, or can be specified
explicitly using --logic-id or --guid flags.

Active states cannot be renamed while locked. Tombstoned states can always be renamed.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newLogicID := args[0]
		if newLogicID == "" {
			return fmt.Errorf("new logic ID cannot be empty")
		}

		// Resolve state reference from flags or .grid context
		explicitRef := dirctx.StateRef{LogicID: renameLogicID, GUID: renameGUID}

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

		// Call SDK RenameState
		result, err := gridClient.RenameState(ctx, sdk.RenameStateInput{
			State: sdk.StateReference{
				GUID:    resolved.GUID,
				LogicID: resolved.LogicID,
			},
			NewLogicID: newLogicID,
		})
		if err != nil {
			return fmt.Errorf("failed to rename state: %w", err)
		}

		// Display results
		pterm.Success.Printf("State renamed successfully\n")
		pterm.Info.Printf("GUID:         %s (unchanged)\n", result.GUID)
		pterm.Info.Printf("Old logic ID: %s\n", result.OldLogicID)
		pterm.Info.Printf("New logic ID: %s\n", result.NewLogicID)
		pterm.Info.Printf("Renamed at:   %s\n", result.RenamedAt.Format(time.RFC3339))

		// Update .grid context if it exists and matches the renamed state
		if gridCtx, err := dirctx.ReadGridContext(); err == nil && gridCtx != nil {
			if gridCtx.StateGUID == result.GUID {
				pterm.Info.Printf("\nUpdating .grid context with new logic ID...\n")
				gridCtx.StateLogicID = result.NewLogicID
				if err := dirctx.WriteGridContext(gridCtx); err != nil {
					pterm.Warning.Printf("Warning: failed to update .grid context: %v\n", err)
				} else {
					pterm.Success.Printf(".grid context updated\n")
				}
			}
		}

		return nil
	},
}

func init() {
	renameCmd.Flags().StringVar(&renameLogicID, "logic-id", "", "Current state logic ID (overrides context)")
	renameCmd.Flags().StringVar(&renameGUID, "guid", "", "State GUID (overrides context)")
}
