package auth

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/client"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := auth.NewFileStore()
		if err != nil {
			return fmt.Errorf("failed to create credential store: %w", err)
		}

		creds, err := store.LoadCredentials()
		if err != nil {
			return fmt.Errorf("not logged in")
		}

		pterm.DefaultSection.Println("Authentication Status")
		pterm.Info.Printf("Logged in with token expiring at: %s\n", creds.ExpiresAt.Format(time.RFC1123))

		// Check if we have a principal ID stored
		if creds.PrincipalID == "" {
			return fmt.Errorf("no principal ID found in credentials (please re-login)")
		}

		pterm.Info.Printf("Principal ID: %s\n", creds.PrincipalID)

		httpClient, err := client.NewAuthenticatedClient(ServerURL)
		if err != nil {
			return err
		}

		gridClient := sdk.NewClient(ServerURL, sdk.WithHTTPClient(httpClient))

		result, err := gridClient.GetEffectivePermissions(cmd.Context(), sdk.GetEffectivePermissionsInput{
			PrincipalID: creds.PrincipalID,
		})
		if err != nil {
			return fmt.Errorf("failed to get effective permissions: %w", err)
		}

		pterm.DefaultSection.Println("Effective Permissions")
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ROLES\tACTIONS\tSCOPE EXPRESSIONS")
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			strings.Join(result.Permissions.Roles, ", "),
			strings.Join(result.Permissions.Actions, ", "),
			strings.Join(result.Permissions.LabelScopeExprs, ", "),
		)
		w.Flush()

		return nil
	},
}
