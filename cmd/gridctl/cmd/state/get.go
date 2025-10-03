package state

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var getCmd = &cobra.Command{
	Use:   "get <guid|logic-id>",
	Short: "Get details of a Terraform state",
	Long:  `Retrieves metadata and backend configuration for a Terraform state using either its GUID or logic ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		identifier := args[0]

		// Create SDK client
		client := sdk.NewClient(ServerURL)

		// Call GetState - the SDK will determine if it's a GUID or logic ID
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Try as logic ID first (most common case)
		state, err := client.GetState(ctx, sdk.StateReference{LogicID: identifier})
		if err != nil {
			// If that fails, try as GUID
			state, err = client.GetState(ctx, sdk.StateReference{GUID: identifier})
			if err != nil {
				return fmt.Errorf("failed to get state: %w", err)
			}
		}

		// Print state details
		fmt.Printf("GUID:     %s\n", state.GUID)
		fmt.Printf("Logic ID: %s\n", state.LogicID)
		fmt.Printf("\nTerraform HTTP Backend endpoints:\n")
		fmt.Printf("  Address: %s\n", state.BackendConfig.Address)
		fmt.Printf("  Lock:    %s\n", state.BackendConfig.LockAddress)
		fmt.Printf("  Unlock:  %s\n", state.BackendConfig.UnlockAddress)

		return nil
	},
}
