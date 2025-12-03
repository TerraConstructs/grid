package state

import (
	"context"
	"fmt"
	"os"
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
	Use:   "get-schema -k <key> [<logic-id>]",
	Short: "Get JSON Schema for a state output",
	Long: `Retrieves the JSON Schema definition for a specific state output.
Returns empty if no schema has been set.
Uses .grid context if no state identifier is provided.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		if getSchemaOutputKey == "" {
			return fmt.Errorf("--key/-k is required")
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

		// Get state info to access OutputKey metadata (includes schema_source)
		info, err := gridClient.GetStateInfo(ctx, sdk.StateReference{
			LogicID: stateRef.LogicID,
			GUID:    stateRef.GUID,
		})
		if err != nil {
			return fmt.Errorf("failed to get state info: %w", err)
		}

		// Find matching output
		var outputKey *sdk.OutputKey
		for _, out := range info.Outputs {
			if out.Key == getSchemaOutputKey {
				outputKey = &out
				break
			}
		}

		if outputKey == nil || outputKey.SchemaJSON == nil {
			fmt.Printf("No schema set for output '%s' on state '%s'\n", getSchemaOutputKey, stateRef.LogicID)
			return nil
		}

		// Display with schema source badge
		schemaSource := "unknown"
		if outputKey.SchemaSource != nil {
			schemaSource = *outputKey.SchemaSource
		}

		fmt.Fprintf(os.Stderr, "Schema (%s):\n", schemaSource)
		fmt.Println(*outputKey.SchemaJSON)

		return nil
	},
}

func init() {
	getOutputSchemaCmd.Flags().StringVar(&getSchemaLogicID, "logic-id", "", "State logic ID")
	getOutputSchemaCmd.Flags().StringVar(&getSchemaGUID, "guid", "", "State GUID")
	getOutputSchemaCmd.Flags().StringVarP(&getSchemaOutputKey, "key", "k", "", "Output key name (required)")
	getOutputSchemaCmd.MarkFlagRequired("key")
}
