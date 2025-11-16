package users

import (
	"bufio"
	"context"
	"fmt"
	"net/mail"
	"os"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
	"golang.org/x/crypto/bcrypt"
)

var (
	emailFlag    string
	usernameFlag string
	passwordFlag string
	rolesInput   []string
	stdinFlag    bool
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new internal IdP user",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate required flags
		if emailFlag == "" {
			return fmt.Errorf("--email flag is required")
		}

		if usernameFlag == "" {
			return fmt.Errorf("--username flag is required")
		}

		if len(rolesInput) == 0 {
			return fmt.Errorf("at least one role must be specified using --role")
		}

		password := passwordFlag
		if stdinFlag {
			// Read password from stdin
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("Enter password: ")
			if scanner.Scan() {
				password = scanner.Text()
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
		}

		if password == "" {
			return fmt.Errorf("password is required (use --password or --stdin)")
		}

		// Validate email format
		if _, err := mail.ParseAddress(emailFlag); err != nil {
			return fmt.Errorf("invalid email format: %w", err)
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cfg.OIDC.ExternalIdP != nil {
			return fmt.Errorf("local users are not supported when using an external identity provider")
		}

		if cfg.OIDC.Issuer == "" {
			return fmt.Errorf("local users require OIDC internal IdP to be enabled (OIDC_ISSUER must be set)")
		}

		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		ctx := context.Background()
		userRepo := repository.NewBunUserRepository(db)
		serviceAccountRepo := repository.NewBunServiceAccountRepository(db)
		sessionRepo := repository.NewBunSessionRepository(db)
		userRoleRepo := repository.NewBunUserRoleRepository(db)
		groupRoleRepo := repository.NewBunGroupRoleRepository(db)
		roleRepo := repository.NewBunRoleRepository(db)
		revokedJTIRepo := repository.NewBunRevokedJTIRepository(db)
		enforcer, err := auth.InitEnforcer(db)
		if err != nil {
			return fmt.Errorf("failed to initialize casbin enforcer: %w", err)
		}
		enforcer.EnableAutoSave(true)

		iamService, err := iam.NewIAMService(
			iam.IAMServiceDependencies{
				Users:           userRepo,
				ServiceAccounts: serviceAccountRepo,
				Sessions:        sessionRepo,
				UserRoles:       userRoleRepo,
				GroupRoles:      groupRoleRepo,
				Roles:           roleRepo,
				RevokedJTIs:     revokedJTIRepo,
				Enforcer:        enforcer,
			},
			iam.IAMServiceConfig{Config: cfg},
		)
		if err != nil {
			return fmt.Errorf("failed to initialize IAM service: %w", err)
		}

		roles, invalidRoles, validRoleNames, err := iamService.GetRolesByName(ctx, rolesInput)
		if err != nil {
			return fmt.Errorf("failed to fetch roles: %w", err)
		}
		if len(invalidRoles) > 0 {
			return fmt.Errorf("invalid role(s): %s\nValid roles are: %s",
				strings.Join(invalidRoles, ", "),
				strings.Join(validRoleNames, ", "))
		}

		// Check if email already exists
		existingUser, err := userRepo.GetByEmail(ctx, emailFlag)
		if err != nil && !isNotFoundError(err) {
			// Real error (not "not found")
			return fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if existingUser != nil {
			return fmt.Errorf("user with email %q already exists", emailFlag)
		}

		// Hash password with bcrypt
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		// Create user with hashed password, Subject=nil (internal IdP marker)
		user := &models.User{
			Email:        emailFlag,
			Name:         usernameFlag,
			PasswordHash: stringPtr(string(hashedPassword)),
			Subject:      nil, // Null subject indicates internal IdP user
		}

		if err := userRepo.Create(ctx, user); err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		// Assign roles to user
		if err := assignRolesToUser(ctx, user, roles, userRoleRepo, enforcer); err != nil {
			return err
		}

		fmt.Println("User created successfully!")
		fmt.Println("----------------------------------------")
		fmt.Printf("User ID: %s\n", user.ID)
		fmt.Printf("Email: %s\n", user.Email)
		fmt.Printf("Username: %s\n", user.Name)
		if len(roles) > 0 {
			roleNames := make([]string, len(roles))
			for i, role := range roles {
				roleNames[i] = role.Name
			}
			fmt.Printf("Roles: %s\n", strings.Join(roleNames, ", "))
		}
		fmt.Println("----------------------------------------")

		return nil
	},
}

func stringPtr(s string) *string {
	return &s
}

// isNotFoundError checks if an error message indicates a "not found" condition
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "user not found")
}

// assignRolesToUser assigns multiple roles to a user
func assignRolesToUser(ctx context.Context, user *models.User, roles []models.Role, userRoleRepo repository.UserRoleRepository, enforcer casbin.IEnforcer) error {
	fmt.Println("Assigning roles...")
	for _, role := range roles {
		userRole := &models.UserRole{
			UserID:     &user.ID,
			RoleID:     role.ID,
			AssignedBy: auth.SystemUserID,
		}

		if err := userRoleRepo.Create(ctx, userRole); err != nil {
			return fmt.Errorf("failed to assign role '%s': %w", role.Name, err)
		}

		// Add Casbin binding for this role
		if _, err := enforcer.AddGroupingPolicy(user.PrincipalSubject(), role.Name); err != nil {
			return fmt.Errorf("failed to add Casbin policy for '%s': %w", role.Name, err)
		}
		fmt.Printf("✓ Assigned role '%s'\n", role.Name)
	}
	return nil
}
