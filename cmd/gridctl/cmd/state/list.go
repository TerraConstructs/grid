package state

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Terraform states",
	Long:  `Lists all Terraform states with status and dependency information.`,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		// Create SDK client
		client := sdk.NewClient(ServerURL)

		// Call ListStates
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		states, err := client.ListStates(ctx)
		if err != nil {
			return fmt.Errorf("failed to list states: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "LOGIC_ID\tGUID\tCOMPUTED_STATUS\tDEPENDENCIES")

		for _, state := range states {
			status := "-"
			if state.ComputedStatus != "" {
				status = state.ComputedStatus
			}

			deps := "-"
			if len(state.DependencyLogicIDs) > 0 {
				logicIDs := append([]string(nil), state.DependencyLogicIDs...)
				sort.Strings(logicIDs)
				deps = strings.Join(logicIDs, ", ")
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", state.LogicID, state.GUID, status, deps)
		}

		w.Flush()

		return nil
	},
}
