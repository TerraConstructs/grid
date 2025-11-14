package iam

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
)

var (
	rolesInput []string
)

// IamCmd is the parent command for iam operations
var IamCmd = &cobra.Command{
	Use:   "iam",
	Short: "Manage External IdP group claims to roles",
	Long:  `Commands for managing iam mappings for External IdP mode.`,
}

func init() {
	IamCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().StringSliceVar(&rolesInput, "role", []string{}, "Role(s) to assign to the group claim")
}

func assignRolesToGroup(ctx context.Context, groupName string, roles []models.Role, iamService iam.Service) error {
	fmt.Println("Assigning roles to group...")

	for _, role := range roles {
		// Use IAM service to assign role (handles DB write, Casbin sync, and cache refresh)
		if err := iamService.AssignGroupRole(ctx, groupName, role.ID); err != nil {
			// Check if it's a duplicate assignment (not a fatal error)
			if strings.Contains(err.Error(), "role already assigned to group") {
				fmt.Printf("  Role '%s' already assigned to group '%s', skipping\n", role.Name, groupName)
				continue
			}
			return fmt.Errorf("failed to assign role '%s' to group '%s': %w", role.Name, groupName, err)
		}

		fmt.Printf("✓ Mapped group '%s' → role '%s' (cache refreshed)\n", groupName, role.Name)
	}
	return nil
}
