package role

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/client"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var removeGroupCmd = &cobra.Command{
	Use:   "remove-group [group] [role]",
	Short: "Remove a group from a role",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		group := args[0]
		role := args[1]

		httpClient, err := client.NewAuthenticatedClient(ServerURL)
		if err != nil {
			return err
		}

		gridClient := sdk.NewClient(ServerURL, sdk.WithHTTPClient(httpClient))

		_, err = gridClient.RemoveGroupRole(cmd.Context(), sdk.RemoveGroupRoleInput{
			GroupName: group,
			RoleName:  role,
		})
		if err != nil {
			return fmt.Errorf("failed to remove group role: %w", err)
		}

		fmt.Printf("Removed group '%s' from role '%s'\n", group, role)
		return nil
	},
}
