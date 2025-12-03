package dep

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
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
Use --state to show incoming dependencies (default) or --from to show outgoing dependents.
If neither flag is provided, .grid context will be used for --state (if available).`,
	Args: cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		consumerLogicID := listConsumerLogicID
		producerLogicID := listProducerLogicID

		// If neither flag provided, try to use .grid context for --state
		if consumerLogicID == "" && producerLogicID == "" {
			gridCtx, err := dirctx.ReadGridContext()
			if err != nil {
				fmt.Printf("Warning: .grid file corrupted or invalid, ignoring: %v\n", err)
				return fmt.Errorf("provide exactly one of --state or --from (no .grid context found)")
			} else if gridCtx != nil {
				consumerLogicID = gridCtx.StateLogicID
				fmt.Printf("Using --state from .grid context: %s\n", consumerLogicID)
			} else {
				return fmt.Errorf("provide exactly one of --state or --from (no .grid context found)")
			}
		}

		showIncoming := consumerLogicID != ""
		showOutgoing := producerLogicID != ""
		if showIncoming == showOutgoing {
			return fmt.Errorf("provide exactly one of --state or --from")
		}

		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 10*time.Second)
		defer cancel()

		var (
			edges  []sdk.DependencyEdge
			header string
		)

		if showIncoming {
			edges, err = gridClient.ListDependencies(ctx, sdk.StateReference{LogicID: consumerLogicID})
			header = fmt.Sprintf("Incoming dependencies for %s", consumerLogicID)
		} else {
			edges, err = gridClient.ListDependents(ctx, sdk.StateReference{LogicID: producerLogicID})
			header = fmt.Sprintf("Outgoing dependents for %s", producerLogicID)
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
		_, _ = fmt.Fprintln(w, "EDGE_ID\tFROM_STATE\tFROM_OUTPUT\tTO_STATE\tTO_INPUT_NAME\tSTATUS\tLAST_UPDATED")
		for _, edge := range edges {
			inputName := "-"
			if edge.ToInputName != "" {
				inputName = edge.ToInputName
			}
			updated := "-"
			if !edge.UpdatedAt.IsZero() {
				updated = edge.UpdatedAt.UTC().Format(time.RFC3339)
			}
			_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				edge.ID,
				edge.From.LogicID,
				edge.FromOutput,
				edge.To.LogicID,
				inputName,
				edge.Status,
				updated,
			)
		}
		_ = w.Flush()
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listConsumerLogicID, "state", "", "Logic ID of consumer state to list incoming dependencies (uses .grid context if not specified)")
	listCmd.Flags().StringVar(&listProducerLogicID, "from", "", "Logic ID of producer state to list outgoing dependents")
}
