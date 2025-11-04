package iam

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

var (
	groupName string
)

// bootstrapCmd creates group→role mappings for external IdP groups
var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap external IdP group→role mappings (External IdP mode)",
	Long: `Create group→role mappings for external IdP (Keycloak, Azure AD, etc.) groups.

This command configures which roles should be granted to users based on their IdP 
group membership.

When a user authenticates:
1. User record is created via Just In Time provisioning
2. Groups are extracted from JWT claims (e.g., groups=["platform-engineers"])
3. Group→role mappings are looked up from database
4. Roles are dynamically applied via Casbin for that session

Use cases:
- Map organization groups to Grid roles (e.g., "test-admins" → platform-engineer role)
- Integration testing with pre-configured group permissions

Example:
  gridapi iam bootstrap \
    --group "test-admins" \
    --role platform-engineer
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Validate required flags
		if groupName == "" {
			return fmt.Errorf("--group is required (external IdP group name from JWT)")
		}
		if len(rolesInput) == 0 {
			return fmt.Errorf("at least one --role must be specified")
		}

		// Load config to get database URL
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Connect to database
		bunDB, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(bunDB)

		// Initialize repositories (only group_roles and roles repos needed)
		roleRepo := repository.NewBunRoleRepository(bunDB)
		groupRoleRepo := repository.NewBunGroupRoleRepository(bunDB)

		// Validate roles
		roles, err := validateRoles(ctx, roleRepo, rolesInput)
		if err != nil {
			return err
		}

		fmt.Printf("Bootstrapping group '%s' → roles %v\n", groupName, rolesInput)

		// Assign roles to group
		if err := assignRolesToGroup(ctx, groupName, roles, groupRoleRepo); err != nil {
			return err
		}

		fmt.Println("✓ Bootstrap complete")
		fmt.Printf("  Group: %s\n", groupName)
		fmt.Printf("  Roles: ")
		for i, role := range roles {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s", role.Name)
		}
		fmt.Println()
		fmt.Println("\nUsers with this group will receive these roles at authentication time.")

		return nil
	},
}

func init() {
	bootstrapCmd.Flags().StringVar(&groupName, "group", "", "External IdP group name (from groups claim in JWT)")
	bootstrapCmd.MarkFlagRequired("group")
}
