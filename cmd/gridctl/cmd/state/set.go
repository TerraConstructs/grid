package state

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	setLogicID   string
	setGUID      string
	setLabelArgs []string
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Apply label updates to an existing state",
	Long: `Adds, updates, or removes labels on an existing state. Repeated --label flags
support key=value for upserts and -key for removals. Defaults to the .grid context when present.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(setLabelArgs) == 0 {
			return fmt.Errorf("at least one --label flag is required")
		}

		adds, removals, warnings, err := parseLabelMutations(setLabelArgs)
		if err != nil {
			return err
		}
		if len(adds) == 0 && len(removals) == 0 {
			return fmt.Errorf("no label mutations provided")
		}
		for _, warning := range warnings {
			pterm.Warning.Println(warning)
		}

		// Resolve state reference from flags, args, or .grid context
		explicitRef := dirctx.StateRef{LogicID: setLogicID, GUID: setGUID}
		if len(args) == 1 && explicitRef.LogicID == "" && explicitRef.GUID == "" {
			explicitRef.LogicID = args[0]
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

		client := sdk.NewClient(ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		stateID := resolved.GUID
		if stateID == "" {
			state, err := client.GetState(ctx, sdk.StateReference{LogicID: resolved.LogicID})
			if err != nil {
				return fmt.Errorf("failed to resolve state GUID for %s: %w", resolved.LogicID, err)
			}
			stateID = state.GUID
		}

		result, err := client.UpdateStateLabels(ctx, sdk.UpdateStateLabelsInput{
			StateID:  stateID,
			Adds:     adds,
			Removals: cloneStringSlice(removals),
		})
		if err != nil {
			return fmt.Errorf("failed to update labels: %w", err)
		}

		pterm.Success.Printf("Updated labels for state %s\n", result.StateID)
		if len(adds) > 0 {
			pterm.Info.Printf("Added/updated: %s\n", formatLabelPreview(adds))
		}
		if len(removals) > 0 {
			pterm.Info.Printf("Removed: %s\n", strings.Join(removals, ","))
		}
		pterm.Info.Printf("Current labels: %s\n", formatLabelPreview(result.Labels))

		return nil
	},
}

func init() {
	setCmd.Flags().StringVar(&setLogicID, "logic-id", "", "State logic ID (overrides context)")
	setCmd.Flags().StringVar(&setGUID, "guid", "", "State GUID (overrides context)")
	setCmd.Flags().StringArrayVar(&setLabelArgs, "label", nil, "Label mutation: key=value to upsert, -key to remove")
}
