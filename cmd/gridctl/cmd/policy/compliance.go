package policy

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var complianceCmd = &cobra.Command{
	Use:   "compliance",
	Short: "Revalidate all states against the current label policy",
	RunE: func(cmd *cobra.Command, args []string) error {
		gridClient, err := sdkClient(cmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		policyResp, err := gridClient.GetLabelPolicy(ctx)
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				pterm.Info.Println("No label policy configured; all states are treated as compliant.")
				return nil
			}
			return fmt.Errorf("failed to get label policy: %w", err)
		}

		definition, err := sdk.ParsePolicyDefinition(policyResp.PolicyJSON)
		if err != nil {
			return fmt.Errorf("invalid policy JSON: %w", err)
		}

		validator := sdk.NewLabelValidator(definition)
		include := true
		states, err := gridClient.ListStatesWithOptions(ctx, sdk.ListStatesOptions{IncludeLabels: &include})
		if err != nil {
			return fmt.Errorf("failed to list states: %w", err)
		}

		type violation struct {
			logicID string
			guid    string
			reason  string
		}

		violations := []violation{}
		for _, state := range states {
			if err := validator.Validate(state.Labels); err != nil {
				violations = append(violations, violation{
					logicID: state.LogicID,
					guid:    state.GUID,
					reason:  err.Error(),
				})
			}
		}

		if len(violations) == 0 {
			pterm.Success.Println("All states comply with the current label policy.")
			return nil
		}

		pterm.Error.Printf("%d state(s) violate the label policy:\n", len(violations))
		table := pterm.TableData{{"LOGIC_ID", "GUID", "REASON"}}
		for _, v := range violations {
			table = append(table, []string{v.logicID, v.guid, v.reason})
		}
		_ = pterm.DefaultTable.WithData(table).Render()
		return fmt.Errorf("label policy compliance failed: %d violation(s)", len(violations))
	},
}
