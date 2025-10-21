package role

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var assignGroupCmd = &cobra.Command{
	Use:   "assign-group [group] [role]",
	Short: "Assign a group to a role",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		group := args[0]
		role := args[1]

		gridClient, err := sdkClient(cmd.Context())
		if err != nil {
			return err
		}

		_, err = gridClient.AssignGroupRole(cmd.Context(), sdk.AssignGroupRoleInput{
			GroupName: group,
			RoleName:  role,
		})
		if err != nil {
			return fmt.Errorf("failed to assign group role: %w", err)
		}

		fmt.Printf("Assigned group '%s' to role '%s'\n", group, role)
		return nil
	},
}
