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
	setSchemaLogicID   string
	setSchemaGUID      string
	setSchemaOutputKey string
	setSchemaFile      string
)

var setOutputSchemaCmd = &cobra.Command{
	Use:   "set-output-schema --key <key> --schema-file <path> [<logic-id>]",
	Short: "Set JSON Schema for a state output",
	Long: `Sets or updates the JSON Schema definition for a specific state output.
This allows declaring expected output types before the output exists in Terraform state.
Uses .grid context if no state identifier is provided.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		if setSchemaOutputKey == "" {
			return fmt.Errorf("--key is required")
		}
		if setSchemaFile == "" {
			return fmt.Errorf("--schema-file is required")
		}

		// Build explicit reference from flags/args
		explicitRef := dirctx.StateRef{}
		if setSchemaLogicID != "" {
			explicitRef.LogicID = setSchemaLogicID
		} else if setSchemaGUID != "" {
			explicitRef.GUID = setSchemaGUID
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

		// Read schema file
		schemaBytes, err := os.ReadFile(setSchemaFile)
		if err != nil {
			return fmt.Errorf("failed to read schema file: %w", err)
		}

		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 10*time.Second)
		defer cancel()

		err = gridClient.SetOutputSchema(ctx, sdk.StateReference{
			LogicID: stateRef.LogicID,
			GUID:    stateRef.GUID,
		}, setSchemaOutputKey, string(schemaBytes))
		if err != nil {
			return fmt.Errorf("failed to set output schema: %w", err)
		}

		fmt.Printf("✓ Set schema for output '%s' on state '%s'\n", setSchemaOutputKey, stateRef.LogicID)
		return nil
	},
}

func init() {
	setOutputSchemaCmd.Flags().StringVar(&setSchemaLogicID, "logic-id", "", "State logic ID")
	setOutputSchemaCmd.Flags().StringVar(&setSchemaGUID, "guid", "", "State GUID")
	setOutputSchemaCmd.Flags().StringVar(&setSchemaOutputKey, "key", "", "Output key name (required)")
	setOutputSchemaCmd.Flags().StringVar(&setSchemaFile, "schema-file", "", "Path to JSON Schema file (required)")
	setOutputSchemaCmd.MarkFlagRequired("key")
	setOutputSchemaCmd.MarkFlagRequired("schema-file")
}
