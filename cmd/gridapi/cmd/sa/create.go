package sa

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	rolesInput []string
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

		roles := make([]*models.Role, len(rolesInput))
		invalidRoles := []string{}
		for i, roleName := range rolesInput {
			role, err := roleRepo.GetByName(ctx, roleName)
			if err != nil {
				invalidRoles = append(invalidRoles, roleName)
				continue
			}
			roles[i] = role
		}
		if len(invalidRoles) > 0 {
			return fmt.Errorf("invalid role(s): %v", invalidRoles)
		}

		// Generate client_id and client_secret
		clientID := uuid.Must(uuid.NewV7()).String()
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			return fmt.Errorf("failed to generate secret: %w", err)
		}
		clientSecret := hex.EncodeToString(secretBytes)

		hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash secret: %w", err)
		}

		sa := &models.ServiceAccount{
			Name:             name,
			ClientID:         clientID,
			ClientSecretHash: string(hashedSecret),
			CreatedBy:        auth.SystemUserID,
		}

		if err := saRepo.Create(ctx, sa); err != nil {
			return fmt.Errorf("failed to create service account: %w", err)
		}

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

		fmt.Println("Service Account created successfully!")
		fmt.Println("----------------------------------------")
		fmt.Printf("Client ID: %s\n", clientID)
		fmt.Printf("Client Secret: %s\n", clientSecret)
		fmt.Println("----------------------------------------")
		fmt.Println("Save the client secret securely. It will not be shown again.")

		return nil
	},
}

func init() {
	SaCmd.AddCommand(createCmd)
	createCmd.Flags().StringSliceVar(&rolesInput, "role", []string{}, "Role(s) to assign to the service account")
}
