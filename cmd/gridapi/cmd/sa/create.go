package sa

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/terraconstructs/grid/cmd/gridapi/cmd/cmdutil"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new service account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if len(rolesInput) == 0 {
			return fmt.Errorf("at least one role must be specified using --role")
		}

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

		bundle, err := cmdutil.NewIAMServiceBundle(cfg, cmdutil.IAMServiceOptions{
			EnableAutoSave: true,
		})
		if err != nil {
			return err
		}
		defer bundle.Close()

		ctx := context.Background()
		iamService := bundle.Service

		roles, invalidRoles, validRoleNames, err := iamService.GetRolesByName(ctx, rolesInput)
		if err != nil {
			return fmt.Errorf("failed to fetch roles: %w", err)
		}
		if len(invalidRoles) > 0 {
			return fmt.Errorf("invalid role(s): %s\nValid roles are: %s",
				strings.Join(invalidRoles, ", "),
				strings.Join(validRoleNames, ", "))
		}

		sa, clientSecret, err := iamService.CreateServiceAccount(ctx, name, auth.SystemUserID)
		if err != nil {
			return fmt.Errorf("failed to create service account: %w", err)
		}

		roleIDs := make([]string, len(roles))
		for i, role := range roles {
			roleIDs[i] = role.ID
		}
		if err := iamService.AssignRolesToServiceAccount(ctx, sa.ID, roleIDs); err != nil {
			return fmt.Errorf("failed to assign roles: %w", err)
		}

		fmt.Println("Service Account created successfully!")
		fmt.Println("----------------------------------------")
		fmt.Printf("Client ID: %s\n", sa.ClientID)
		fmt.Printf("Client Secret: %s\n", clientSecret)
		fmt.Println("----------------------------------------")
		fmt.Println("Save the client secret securely. It will not be shown again.")

		return nil
	},
}
