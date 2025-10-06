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
	getLogicID string
	getGUID    string
)

var getCmd = &cobra.Command{
	Use:   "get [<logic-id>]",
	Short: "Get details of a Terraform state",
	Long: `Retrieves comprehensive information about a Terraform state including dependencies,
dependents, and outputs. Uses .grid context if no identifier is provided.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		// Build explicit reference from flags/args
		explicitRef := dirctx.StateRef{}
		if getLogicID != "" {
			explicitRef.LogicID = getLogicID
		} else if getGUID != "" {
			explicitRef.GUID = getGUID
		} else if len(args) == 1 {
			explicitRef.LogicID = args[0]
		}

		// Try to read .grid context
		contextRef := dirctx.StateRef{}
		gridCtx, err := dirctx.ReadGridContext()
		if err != nil {
			pterm.Warning.Printf("Warning: .grid file corrupted or invalid, ignoring: %v\n", err)
		} else if gridCtx != nil {
			contextRef.LogicID = gridCtx.StateLogicID
			contextRef.GUID = gridCtx.StateGUID
		}

		// Resolve final state reference
		stateRef, err := dirctx.ResolveStateRef(explicitRef, contextRef)
		if err != nil {
			return err
		}

		// Create SDK client
		client := sdk.NewClient(ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Call GetStateInfo for enhanced display
		info, err := client.GetStateInfo(ctx, sdk.StateReference{
			LogicID: stateRef.LogicID,
			GUID:    stateRef.GUID,
		})
		if err != nil {
			return fmt.Errorf("failed to get state info: %w", err)
		}

		// Print state header
		fmt.Printf("State: %s (guid: %s)\n", info.State.LogicID, info.State.GUID)
		fmt.Printf("Created: %s\n", info.CreatedAt.Format("2006-01-02 15:04:05"))
		if !info.UpdatedAt.IsZero() {
			fmt.Printf("Updated: %s\n", info.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()

		// Print dependencies (incoming edges)
		fmt.Println("Dependencies (consuming from):")
		if len(info.Dependencies) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, dep := range info.Dependencies {
				toInput := dep.ToInputName
				if toInput == "" {
					toInput = "(auto-generated)"
				}
				fmt.Printf("  %s.%s → %s\n", dep.From.LogicID, dep.FromOutput, toInput)
			}
		}
		fmt.Println()

		// Print dependents (outgoing edges)
		fmt.Println("Dependents (consumed by):")
		if len(info.Dependents) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, dep := range info.Dependents {
				fmt.Printf("  %s (using %s)\n", dep.To.LogicID, dep.FromOutput)
			}
		}
		fmt.Println()

		// Print outputs
		fmt.Println("Outputs:")
		if len(info.Outputs) == 0 {
			fmt.Println("  (none - no Terraform state uploaded yet)")
		} else {
			for _, out := range info.Outputs {
				sensitive := ""
				if out.Sensitive {
					sensitive = " (⚠️  sensitive)"
				}
				fmt.Printf("  %s%s\n", out.Key, sensitive)
			}
		}
		fmt.Println()

		// Print backend config
		fmt.Println("Terraform HTTP Backend endpoints:")
		fmt.Printf("  Address: %s\n", info.BackendConfig.Address)
		fmt.Printf("  Lock:    %s\n", info.BackendConfig.LockAddress)
		fmt.Printf("  Unlock:  %s\n", info.BackendConfig.UnlockAddress)

		return nil
	},
}

func init() {
	getCmd.Flags().StringVar(&getLogicID, "logic-id", "", "State logic ID (overrides positional arg and context)")
	getCmd.Flags().StringVar(&getGUID, "guid", "", "State GUID (overrides positional arg and context)")
}
