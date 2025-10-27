package iam

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
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

// validateRoles fetches all roles from the repository and validates the provided role names.
// Returns the matching roles or an error with details about invalid roles.
func validateRoles(ctx context.Context, roleRepo repository.RoleRepository, roleNames []string) ([]*models.Role, error) {
	allRoles, err := roleRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	roleMap := make(map[string]*models.Role, len(allRoles))
	validRoleNames := make([]string, 0, len(allRoles))
	for _, role := range allRoles {
		roleMap[role.Name] = &role
		validRoleNames = append(validRoleNames, role.Name)
	}

	roles := make([]*models.Role, 0, len(roleNames))
	invalidRoles := make([]string, 0)

	for _, roleName := range roleNames {
		role, exists := roleMap[roleName]
		if !exists {
			invalidRoles = append(invalidRoles, roleName)
			continue
		}
		roles = append(roles, role)
	}

	if len(invalidRoles) > 0 {
		return nil, fmt.Errorf("invalid role(s): %v\nValid roles are: %v",
			strings.Join(invalidRoles, ", "),
			strings.Join(validRoleNames, ", "))
	}

	return roles, nil
}

func assignRolesToGroup(ctx context.Context, groupName string, roles []*models.Role, groupRoleRepo repository.GroupRoleRepository) error {
	fmt.Println("Assigning roles to group...")

	// Get existing mappings once to check for duplicates
	existing, err := groupRoleRepo.GetByGroupName(ctx, groupName)
	existingRoleIDs := make(map[string]bool)
	if err == nil {
		for _, mapping := range existing {
			existingRoleIDs[mapping.RoleID] = true
		}
	}

	for _, role := range roles {
		// Check if this role is already mapped
		if existingRoleIDs[role.ID] {
			fmt.Printf("  Role '%s' already assigned to group '%s', skipping\n", role.Name, groupName)
			continue
		}

		groupRole := &models.GroupRole{
			GroupName:  groupName,
			RoleID:     role.ID,
			AssignedBy: auth.SystemUserID,
		}

		if err := groupRoleRepo.Create(ctx, groupRole); err != nil {
			return fmt.Errorf("failed to assign role '%s' to group '%s': %w", role.Name, groupName, err)
		}

		// Note: Casbin g2 (group→role) entries are created dynamically at auth time
		// via ApplyDynamicGroupings in authn.go, not during bootstrap
		fmt.Printf("✓ Mapped group '%s' → role '%s'\n", groupName, role.Name)
	}
	return nil
}
