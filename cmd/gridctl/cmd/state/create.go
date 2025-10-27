package state

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	createForce     bool
	createLabelArgs []string
)

var createCmd = &cobra.Command{
	Use:   "create [<logic-id>]",
	Short: "Create a new Terraform state",
	Long: `Creates a new Terraform state with a user-provided logic ID.
A client-generated UUID (v7) is used as the immutable state identifier.
A .grid context file is created in the current directory to remember this state.

If logic-id is not provided, the .grid context will be used (if available).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		// Get logic-id from arg or .grid context
		var logicID string
		if len(args) == 1 {
			logicID = args[0]
		}

		// Check for existing .grid context
		existingCtx, err := dirctx.ReadGridContext()
		if err != nil {
			// Corrupted file - warn but continue
			pterm.Warning.Printf("Warning: .grid file corrupted or invalid, ignoring: %v\n", err)
		}

		// If logic-id not provided, try to use .grid context
		if logicID == "" {
			if existingCtx != nil {
				logicID = existingCtx.StateLogicID
				pterm.Info.Printf("Using logic-id from .grid context: %s\n", logicID)
				pterm.Info.Println("State already created, this will be idempotent (re-create with same ID)")
			} else {
				return fmt.Errorf("logic-id is required (no .grid context found)")
			}
		}

		// If .grid exists and logic-id is different, warn user
		if existingCtx != nil && existingCtx.StateLogicID != logicID && !createForce {
			return fmt.Errorf(".grid exists for state %s (GUID: %s); use --force to overwrite with %s",
				existingCtx.StateLogicID, existingCtx.StateGUID, logicID)
		}
		if existingCtx != nil && existingCtx.StateLogicID != logicID && createForce {
			pterm.Info.Printf("Replacing existing .grid context (was: %s, now: %s)\n", existingCtx.StateLogicID, logicID)
		}

		// Parse label flags (if any)
		labels, warnings, err := parseLabelArgs(createLabelArgs)
		if err != nil {
			return err
		}
		for _, warning := range warnings {
			pterm.Warning.Println(warning)
		}

		// Generate UUIDv7 for the state
		guid := uuid.Must(uuid.NewV7()).String()

		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}

		// Call CreateState with labels
		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 10*time.Second)
		defer cancel()

		state, err := gridClient.CreateState(ctx, sdk.CreateStateInput{
			GUID:    guid,
			LogicID: logicID,
			Labels:  labels,
		})
		if err != nil {
			return fmt.Errorf("failed to create state: %w", err)
		}

		// Print success with GUID and backend config endpoints
		fmt.Printf("Created state: %s\n", state.GUID)
		fmt.Printf("Logic ID: %s\n", state.LogicID)
		fmt.Printf("\nTerraform HTTP Backend endpoints:\n")
		fmt.Printf("  Address: %s\n", state.BackendConfig.Address)
		fmt.Printf("  Lock:    %s\n", state.BackendConfig.LockAddress)
		fmt.Printf("  Unlock:  %s\n", state.BackendConfig.UnlockAddress)

		// Write .grid context file
		now := time.Now()
		gridCtx := &dirctx.DirectoryContext{
			Version:      dirctx.GridFileVersion,
			StateGUID:    state.GUID,
			StateLogicID: state.LogicID,
			ServerURL:    ServerURL,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := dirctx.WriteGridContext(gridCtx); err != nil {
			// Non-fatal: warn but don't fail the command
			pterm.Warning.Printf("Warning: Cannot write .grid file (permission denied?), state context will not be saved: %v\n", err)
			pterm.Info.Println("State created successfully, but you'll need to specify --logic-id for subsequent commands")
		} else {
			pterm.Success.Printf("Saved state context to .grid file\n")
			pterm.Info.Println("Subsequent commands in this directory will use this state automatically")
		}

		return nil
	},
}

func init() {
	createCmd.Flags().BoolVar(&createForce, "force", false, "Overwrite existing .grid context file")
	createCmd.Flags().StringArrayVar(&createLabelArgs, "label", nil, "Apply label (key=value). Repeatable. Prefix with -key to remove is unsupported for create")
}
