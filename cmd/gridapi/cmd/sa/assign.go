package sa

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

var assignCmd = &cobra.Command{
	Use:   "assign [name]",
	Short: "Assign roles to a service account",
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

		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		ctx := context.Background()
		saRepo := repository.NewBunServiceAccountRepository(db)
		roleRepo := repository.NewBunRoleRepository(db)
		userRoleRepo := repository.NewBunUserRoleRepository(db)
		enforcer, err := auth.InitEnforcer(db, cfg.CasbinModelPath)
		if err != nil {
			return fmt.Errorf("failed to initialize casbin enforcer: %w", err)
		}
		enforcer.EnableAutoSave(true)

		roles, err := validateRoles(ctx, roleRepo, rolesInput)
		if err != nil {
			return err
		}

		sa, err := saRepo.GetByName(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to fetch service account: %w", err)
		}

		if err := assignRolesToServiceAccount(ctx, sa, roles, userRoleRepo, enforcer); err != nil {
			return err
		}

		return nil
	},
}
