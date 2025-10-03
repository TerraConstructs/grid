package deps

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var removeEdgeID int64

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a dependency edge",
	Long:  `Removes a dependency edge by its ID. Use 'deps list' to find edge IDs.`,
	Args:  cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		if removeEdgeID <= 0 {
			return fmt.Errorf("flag --edge-id must be provided")
		}

		client := sdk.NewClient(ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.RemoveDependency(ctx, removeEdgeID); err != nil {
			return fmt.Errorf("failed to remove dependency: %w", err)
		}

		fmt.Printf("Dependency removed (edge ID: %d)\n", removeEdgeID)
		return nil
	},
}

func init() {
	removeCmd.Flags().Int64Var(&removeEdgeID, "edge-id", 0, "Edge ID to remove")
}
