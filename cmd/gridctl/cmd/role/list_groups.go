package role

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	internalclient "github.com/terraconstructs/grid/cmd/gridctl/internal/client"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var listGroupsCmd = &cobra.Command{
	Use:   "list-groups [group]",
	Short: "List group-to-role assignments",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var group string
		if len(args) > 0 {
			group = args[0]
		}

		httpClient, err := internalclient.NewAuthenticatedClient(ServerURL)
		if err != nil {
			return err
		}

		gridClient := sdk.NewClient(ServerURL, sdk.WithHTTPClient(httpClient))

		result, err := gridClient.ListGroupRoles(cmd.Context(), sdk.ListGroupRolesInput{
			GroupName: group,
		})
		if err != nil {
			return fmt.Errorf("failed to list group roles: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "GROUP\tROLE\tASSIGNED AT\tASSIGNED BY")
		for _, a := range result.Assignments {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.GroupName, a.RoleName, a.AssignedAt.Format(time.RFC3339), a.AssignedByUserID)
		}
		w.Flush()

		return nil
	},
}
