package sa

import (
	"context"
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

var (
	rolesInput []string
)

// SaCmd is the parent command for service account operations
var SaCmd = &cobra.Command{
	Use:   "sa",
	Short: "Manage service accounts",
	Long:  `Commands for managing service accounts directly from the server.`,
}

func init() {
	SaCmd.AddCommand(listCmd)
	SaCmd.AddCommand(createCmd)
	createCmd.Flags().StringSliceVar(&rolesInput, "role", []string{}, "Role(s) to assign to the service account")
	SaCmd.AddCommand(assignCmd)
	assignCmd.Flags().StringSliceVar(&rolesInput, "role", []string{}, "Role(s) to assign to the service account")
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

func assignRolesToServiceAccount(ctx context.Context, sa *models.ServiceAccount, roles []*models.Role, userRoleRepo repository.UserRoleRepository, enforcer casbin.IEnforcer) error {
	fmt.Println("Assigning roles...")
	for _, role := range roles {
		userRole := &models.UserRole{
			ServiceAccountID: &sa.ID,
			RoleID:           role.ID,
			AssignedBy:       auth.SystemUserID,
		}

		if err := userRoleRepo.Create(ctx, userRole); err != nil {
			return fmt.Errorf("failed to assign role '%s': %w", role.Name, err)
		}

		casbinPrincipalID := auth.ServiceAccountID(sa.ClientID)
		casbinRoleID := auth.RoleID(role.Name)
		if _, err := enforcer.AddRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
			return fmt.Errorf("failed to add casbin role assignment for '%s': %w", role.Name, err)
		}
		fmt.Printf("✓ Assigned role '%s'\n", role.Name)
	}
	return nil
}

func unassignRolesFromServiceAccount(ctx context.Context, sa *models.ServiceAccount, roles []*models.Role, userRoleRepo repository.UserRoleRepository, enforcer casbin.IEnforcer) error {
	fmt.Println("Unassigning roles...")
	for _, role := range roles {
		userRole, err := userRoleRepo.GetByServiceAccountAndRoleID(ctx, sa.ID, role.ID)
		if err != nil {
			return fmt.Errorf("failed to fetch user role for unassignment of role '%s': %w", role.Name, err)
		}

		if err := userRoleRepo.Delete(ctx, userRole.ID); err != nil {
			return fmt.Errorf("failed to unassign role '%s': %w", role.Name, err)
		}

		casbinPrincipalID := auth.ServiceAccountID(sa.ClientID)
		casbinRoleID := auth.RoleID(role.Name)
		if _, err := enforcer.DeleteRoleForUser(casbinPrincipalID, casbinRoleID); err != nil {
			return fmt.Errorf("failed to remove casbin role assignment for '%s': %w", role.Name, err)
		}
		fmt.Printf("✓ Unassigned role '%s'\n", role.Name)
	}
	return nil
}
