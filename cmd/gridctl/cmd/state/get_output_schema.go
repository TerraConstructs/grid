package state

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	getSchemaLogicID   string
	getSchemaGUID      string
	getSchemaOutputKey string
)

var getOutputSchemaCmd = &cobra.Command{
	Use:   "get-output-schema --output-key <key> [<logic-id>]",
	Short: "Get JSON Schema for a state output",
	Long: `Retrieves the JSON Schema definition for a specific state output.
Returns empty if no schema has been set.
Uses .grid context if no state identifier is provided.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		if getSchemaOutputKey == "" {
			return fmt.Errorf("--output-key is required")
		}

		// Build explicit reference from flags/args
		explicitRef := dirctx.StateRef{}
		if getSchemaLogicID != "" {
			explicitRef.LogicID = getSchemaLogicID
		} else if getSchemaGUID != "" {
			explicitRef.GUID = getSchemaGUID
		} else if len(args) == 1 {
			explicitRef.LogicID = args[0]
		}

		// Try to read .grid context
		contextRef := dirctx.StateRef{}
		gridCtx, err := dirctx.ReadGridContext()
		if err == nil && gridCtx != nil {
			contextRef.LogicID = gridCtx.StateLogicID
			contextRef.GUID = gridCtx.StateGUID
		}

		// Resolve final state reference
		stateRef, err := dirctx.ResolveStateRef(explicitRef, contextRef)
		if err != nil {
			return err
		}

		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 10*time.Second)
		defer cancel()

		schemaJSON, err := gridClient.GetOutputSchema(ctx, sdk.StateReference{
			LogicID: stateRef.LogicID,
			GUID:    stateRef.GUID,
		}, getSchemaOutputKey)
		if err != nil {
			return fmt.Errorf("failed to get output schema: %w", err)
		}

		if schemaJSON == "" {
			fmt.Printf("No schema set for output '%s' on state '%s'\n", getSchemaOutputKey, stateRef.LogicID)
		} else {
			fmt.Println(schemaJSON)
		}

		return nil
	},
}

func init() {
	getOutputSchemaCmd.Flags().StringVar(&getSchemaLogicID, "logic-id", "", "State logic ID")
	getOutputSchemaCmd.Flags().StringVar(&getSchemaGUID, "guid", "", "State GUID")
	getOutputSchemaCmd.Flags().StringVar(&getSchemaOutputKey, "output-key", "", "Output key name (required)")
	getOutputSchemaCmd.MarkFlagRequired("output-key")
}
