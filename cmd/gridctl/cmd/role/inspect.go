package role

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [principal_id]",
	Short: "Inspect a principal's effective permissions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		principalID := args[0]

		gridClient, err := sdkClient(cmd.Context())
		if err != nil {
			return err
		}

		result, err := gridClient.GetEffectivePermissions(cmd.Context(), sdk.GetEffectivePermissionsInput{
			PrincipalID: principalID,
		})
		if err != nil {
			return fmt.Errorf("failed to get effective permissions: %w", err)
		}

		fmt.Printf("Principal: %s\n", principalID)
		fmt.Println("Roles:")
		for _, role := range result.Permissions.Roles {
			fmt.Printf("  - %s\n", role)
		}
		fmt.Println("Actions:")
		for _, action := range result.Permissions.Actions {
			fmt.Printf("  - %s\n", action)
		}
		fmt.Println("Label Scope Expressions:")
		for _, expr := range result.Permissions.LabelScopeExprs {
			fmt.Printf("  - %s\n", expr)
		}
		// TODO: Print create constraints and immutable keys

		return nil
	},
}
