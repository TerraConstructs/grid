package iam

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/terraconstructs/grid/cmd/gridapi/cmd/cmdutil"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
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

		bundle, err := cmdutil.NewIAMServiceBundle(cfg, cmdutil.IAMServiceOptions{
			EnableAutoSave: false,
		})
		if err != nil {
			return err
		}
		defer bundle.Close()

		// Fetch roles via service layer to ensure consistent validation
		roles, invalidRoles, validRoleNames, err := bundle.Service.GetRolesByName(ctx, rolesInput)
		if err != nil {
			return fmt.Errorf("failed to fetch roles: %w", err)
		}
		if len(invalidRoles) > 0 {
			return fmt.Errorf("invalid role(s): %s\nValid roles are: %s",
				strings.Join(invalidRoles, ", "),
				strings.Join(validRoleNames, ", "))
		}

		fmt.Printf("Bootstrapping group '%s' → roles %v\n", groupName, rolesInput)

		// Assign roles to group via IAM service (auto-refreshes cache)
		if err := assignRolesToGroup(ctx, groupName, roles, bundle.Service); err != nil {
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
		fmt.Println("\nSend SIGHUP to running gridapi processes to reload group→role mappings!")
		fmt.Println("  Example: pkill -SIGHUP gridapi")

		return nil
	},
}

func init() {
	bootstrapCmd.Flags().StringVar(&groupName, "group", "", "External IdP group name (from groups claim in JWT)")
	_ = bootstrapCmd.MarkFlagRequired("group")
}
