package dep

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var removeEdgeID int64

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a dependency edge",
	Long:  `Removes a dependency edge by its ID. Use 'dep list' to find edge IDs.`,
	Args:  cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		if removeEdgeID <= 0 {
			return fmt.Errorf("flag --id/-i must be provided")
		}

		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 10*time.Second)
		defer cancel()

		if err := gridClient.RemoveDependency(ctx, removeEdgeID); err != nil {
			return fmt.Errorf("failed to remove dependency: %w", err)
		}

		fmt.Printf("Dependency removed (edge ID: %d)\n", removeEdgeID)
		return nil
	},
}

func init() {
	removeCmd.Flags().Int64VarP(&removeEdgeID, "id", "i", 0, "Edge ID to remove")
}
