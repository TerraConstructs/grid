package state

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Terraform states",
	Long:  `Lists all Terraform states with status and dependency information.`,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}

		// Call ListStates
		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 10*time.Second)
		defer cancel()

		labelsFilter, warnings, err := parseLabelArgs(listLabelFilterArgs)
		if err != nil {
			return err
		}
		for _, warning := range warnings {
			pterm.Warning.Println(warning)
		}

		finalFilter := strings.TrimSpace(listFilter)
		if len(labelsFilter) > 0 {
			labelExpr := sdk.BuildBexprFilter(labelsFilter)
			if finalFilter == "" {
				finalFilter = labelExpr
			} else {
				finalFilter = fmt.Sprintf("(%s) && (%s)", finalFilter, labelExpr)
			}
		}

		include := true
		states, err := gridClient.ListStatesWithOptions(ctx, sdk.ListStatesOptions{Filter: finalFilter, IncludeLabels: &include})
		if err != nil {
			return fmt.Errorf("failed to list states: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "LOGIC_ID\tGUID\tLABELS\tCOMPUTED_STATUS\tDEPENDENCIES")

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

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", state.LogicID, state.GUID, formatLabelPreview(state.Labels), status, deps)
		}

		w.Flush()

		return nil
	},
}

var (
	listFilter          string
	listLabelFilterArgs []string
)

func init() {
	listCmd.Flags().StringVar(&listFilter, "filter", "", "bexpr filter expression (e.g. env == \"prod\")")
	listCmd.Flags().StringArrayVar(&listLabelFilterArgs, "label", nil, "Filter by label equality (key=value). Converted to bexpr AND expression")
}
