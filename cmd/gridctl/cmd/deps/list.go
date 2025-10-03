package deps

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	listConsumerLogicID string
	listProducerLogicID string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List dependencies for a state",
	Long: `Lists all incoming dependency edges (dependencies) or outgoing edges (dependents) for a state.
Use --state to show incoming dependencies (default) or --from to show outgoing dependents. Exactly one flag must be provided.`,
	Args: cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		showIncoming := listConsumerLogicID != ""
		showOutgoing := listProducerLogicID != ""
		if showIncoming == showOutgoing {
			return fmt.Errorf("provide exactly one of --state or --from")
		}

		client := sdk.NewClient(ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var (
			edges  []sdk.DependencyEdge
			err    error
			header string
		)

		if showIncoming {
			edges, err = client.ListDependencies(ctx, sdk.StateReference{LogicID: listConsumerLogicID})
			header = fmt.Sprintf("Incoming dependencies for %s", listConsumerLogicID)
		} else {
			edges, err = client.ListDependents(ctx, sdk.StateReference{LogicID: listProducerLogicID})
			header = fmt.Sprintf("Outgoing dependents for %s", listProducerLogicID)
		}
		if err != nil {
			if showIncoming {
				return fmt.Errorf("failed to list dependencies: %w", err)
			}
			return fmt.Errorf("failed to list dependents: %w", err)
		}

		if len(edges) == 0 {
			fmt.Println("No edges found")
			return nil
		}

		sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })

		fmt.Println(header)
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "EDGE_ID\tFROM_STATE\tFROM_OUTPUT\tTO_STATE\tTO_INPUT_NAME\tSTATUS\tLAST_UPDATED")
		for _, edge := range edges {
			inputName := "-"
			if edge.ToInputName != "" {
				inputName = edge.ToInputName
			}
			updated := "-"
			if !edge.UpdatedAt.IsZero() {
				updated = edge.UpdatedAt.UTC().Format(time.RFC3339)
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				edge.ID,
				edge.From.LogicID,
				edge.FromOutput,
				edge.To.LogicID,
				inputName,
				edge.Status,
				updated,
			)
		}
		w.Flush()
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listConsumerLogicID, "state", "", "Logic ID of consumer state to list incoming dependencies")
	listCmd.Flags().StringVar(&listProducerLogicID, "from", "", "Logic ID of producer state to list outgoing dependents")
}
