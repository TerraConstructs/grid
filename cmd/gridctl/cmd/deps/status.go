package deps

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var statusLogicID string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show computed dependency status for a state",
	Long: `Computes and displays the dependency status for a state.
Status can be:
  - clean: All incoming dependencies are clean
  - dirty: At least one incoming dependency has changed (red/yellow propagation)
  - pending: At least one incoming dependency is pending
  - missing-output: At least one incoming dependency references a non-existent output
  - unknown: Unable to determine status`,
	Args: cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		logicID := statusLogicID
		if strings.TrimSpace(logicID) == "" {
			gridCtx, err := dirctx.ReadGridContext()
			if err != nil {
				fmt.Printf("Warning: .grid file corrupted or invalid, ignoring: %v\n", err)
				return fmt.Errorf("flag --state is required (no .grid context found)")
			} else if gridCtx != nil {
				logicID = gridCtx.StateLogicID
				fmt.Printf("Using --state from .grid context: %s\n", logicID)
			} else {
				return fmt.Errorf("flag --state is required (no .grid context found)")
			}
		}

		client := sdk.NewClient(ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := client.GetStateStatus(ctx, sdk.StateReference{LogicID: logicID})
		if err != nil {
			return fmt.Errorf("failed to get state status: %w", err)
		}

		fmt.Printf("State: %s (%s)\n", status.State.LogicID, status.State.GUID)
		fmt.Printf("Computed status: %s\n\n", status.Status)

		fmt.Println("Incoming edges summary:")
		fmt.Printf("  Clean:   %d\n", status.Summary.IncomingClean)
		fmt.Printf("  Dirty:   %d\n", status.Summary.IncomingDirty)
		fmt.Printf("  Pending: %d\n", status.Summary.IncomingPending)
		fmt.Printf("  Unknown: %d\n", status.Summary.IncomingUnknown)

		if len(status.Incoming) == 0 {
			fmt.Println("\nNo incoming dependency edges recorded.")
			return nil
		}

		fmt.Println("\nIncoming edge details:")
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "EDGE_ID\tFROM_STATE\tOUTPUT\tSTATUS\tPRODUCER_DIGEST\tCONSUMER_DIGEST\tLAST_PRODUCED\tLAST_CONSUMED")
		for _, edge := range status.Incoming {
			producerDigest := "-"
			if edge.InDigest != "" {
				producerDigest = edge.InDigest
			}
			consumerDigest := "-"
			if edge.OutDigest != "" {
				consumerDigest = edge.OutDigest
			}
			lastProduced := "-"
			if edge.LastProducedAt != nil {
				lastProduced = edge.LastProducedAt.UTC().Format(time.RFC3339)
			}
			lastConsumed := "-"
			if edge.LastConsumedAt != nil {
				lastConsumed = edge.LastConsumedAt.UTC().Format(time.RFC3339)
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				edge.ID,
				edge.From.LogicID,
				edge.FromOutput,
				edge.Status,
				producerDigest,
				consumerDigest,
				lastProduced,
				lastConsumed,
			)
		}
		w.Flush()
		return nil
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusLogicID, "state", "", "Logic ID of the state to inspect")
}
