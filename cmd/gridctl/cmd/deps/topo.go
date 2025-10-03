package deps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	topoLogicID   string
	topoDirection string
)

var topoCmd = &cobra.Command{
	Use:   "topo",
	Short: "Show topological order for a state",
	Long: `Computes and displays the topological order (layered graph) starting from a state.
Direction can be:
  - upstream: Show all producers (dependencies) in layers
  - downstream: Show all consumers (dependents) in layers`,
	Args: cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		if topoLogicID == "" {
			return fmt.Errorf("flag --state is required")
		}

		direction := strings.ToLower(strings.TrimSpace(topoDirection))
		if direction == "" {
			direction = "downstream"
		}
		if direction != "downstream" && direction != "upstream" {
			return fmt.Errorf("direction must be 'downstream' or 'upstream'")
		}

		client := sdk.NewClient(ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		dir := sdk.Downstream
		if direction == string(sdk.Upstream) {
			dir = sdk.Upstream
		}

		layers, err := client.GetTopologicalOrder(ctx, sdk.TopologyInput{
			Root:      sdk.StateReference{LogicID: topoLogicID},
			Direction: dir,
		})
		if err != nil {
			return fmt.Errorf("failed to get topological order: %w", err)
		}

		if len(layers) == 0 {
			fmt.Println("No layers found")
			return nil
		}

		fmt.Printf("Topological order (%s):\n", direction)
		for _, layer := range layers {
			fmt.Printf("Layer %d:\n", layer.Level)
			for _, state := range layer.States {
				logicID := state.LogicID
				if logicID == "" {
					logicID = state.GUID
				}
				fmt.Printf("  - %s (%s)\n", logicID, state.GUID)
			}
		}
		return nil
	},
}

func init() {
	topoCmd.Flags().StringVar(&topoLogicID, "state", "", "Logic ID of the state to inspect")
	topoCmd.Flags().StringVar(&topoDirection, "direction", "downstream", "Traversal direction: upstream or downstream")
}
