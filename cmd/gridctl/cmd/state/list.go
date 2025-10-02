package state

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Terraform states",
	Long:  `Lists all Terraform states in tab-delimited format (GUID -> logic_id).`,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		// Create SDK client
		client := sdk.NewClient(http.DefaultClient, ServerURL)

		// Call ListStates
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.ListStates(ctx)
		if err != nil {
			return fmt.Errorf("failed to list states: %w", err)
		}

		// Print tab-delimited output
		for _, state := range resp.States {
			fmt.Printf("%s\t%s\n", state.Guid, state.LogicId)
		}

		return nil
	},
}
