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

var searchOutputKey string

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for dependencies by output key",
	Long:  `Searches for all dependency edges that reference a specific output key.`,
	Args:  cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		if searchOutputKey == "" {
			return fmt.Errorf("flag --output is required")
		}

		client := sdk.NewClient(ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		edges, err := client.SearchByOutput(ctx, searchOutputKey)
		if err != nil {
			return fmt.Errorf("failed to search by output: %w", err)
		}

		if len(edges) == 0 {
			fmt.Printf("No edges found with output key: %s\n", searchOutputKey)
			return nil
		}

		sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "EDGE_ID\tFROM_STATE\tFROM_OUTPUT\tTO_STATE\tTO_INPUT_NAME\tSTATUS\tUPDATED_AT\n")
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
	searchCmd.Flags().StringVar(&searchOutputKey, "output", "", "Producer output key to search for")
}
