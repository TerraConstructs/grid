package sa

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List service accounts with their roles",
	RunE: func(cmd *cobra.Command, args []string) error {

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cfg.OIDC.ExternalIdP != nil {
			return fmt.Errorf("local service accounts are not supported when using an external identity provider")
		}

		if cfg.OIDC.Issuer == "" {
			return fmt.Errorf("local service accounts require OIDC internal IdP to be enabled (OIDC_ISSUER must be set)")
		}

		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		ctx := context.Background()
		saRepo := repository.NewBunServiceAccountRepository(db)

		roleRepo := repository.NewBunRoleRepository(db)
		userRoleRepo := repository.NewBunUserRoleRepository(db)

		// TODO: Fix n+1 by using Bun Relations between tables...
		serviceAccounts, err := saRepo.List(ctx)
		if err != nil {
			return fmt.Errorf("failed to list service accounts: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tCLIENT_ID\tCREATED_AT\tROLES\tDISABLED")
		for _, sa := range serviceAccounts {
			userRoles, err := userRoleRepo.GetByServiceAccountID(ctx, sa.ID)
			if err != nil {
				return fmt.Errorf("failed to list roles for service account '%s': %w", sa.ID, err)
			}
			roles := make([]string, len(userRoles))
			for i, userRole := range userRoles {
				role, err := roleRepo.GetByID(ctx, userRole.RoleID)
				if err != nil {
					return fmt.Errorf("failed to get role '%s': %w", userRole.RoleID, err)
				}
				roles[i] = role.Name
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\n",
				sa.Name,
				sa.ClientID,
				sa.CreatedAt,
				strings.Join(roles, ", "),
				sa.Disabled,
			)
		}
		w.Flush()

		return nil
	},
}
