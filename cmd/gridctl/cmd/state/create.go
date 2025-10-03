package state

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var createCmd = &cobra.Command{
	Use:   "create <logic-id>",
	Short: "Create a new Terraform state",
	Long: `Creates a new Terraform state with a user-provided logic ID.
A client-generated UUID (v7) is used as the immutable state identifier.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		logicID := args[0]

		// Generate UUIDv7 for the state
		guid := uuid.Must(uuid.NewV7()).String()

		// Create SDK client
		client := sdk.NewClient(ServerURL)

		// Call CreateState
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		state, err := client.CreateState(ctx, sdk.CreateStateInput{
			GUID:    guid,
			LogicID: logicID,
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

		return nil
	},
}
