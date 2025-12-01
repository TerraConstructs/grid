package migrations

import (
	"context"
	"fmt"

	casbinbunadapter "github.com/terraconstructs/grid/cmd/gridapi/internal/auth/bunadapter"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251013140500, down_20251013140500)
}

// up_20251013140500 creates auth tables for authentication, authorization, and RBAC
func up_20251013140500(ctx context.Context, db *bun.DB) error {
	// 1. Create users table
	fmt.Print(" [up] creating users table...")
	_, err := db.NewCreateTable().
		Model((*models.User)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create indexes for users
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email)`)
	if err != nil {
		return fmt.Errorf("failed to create users email index: %w", err)
	}
	fmt.Println(" OK")

	// 2. Create service_accounts table
	fmt.Print(" [up] creating service_accounts table...")
	_, err = db.NewCreateTable().
		Model((*models.ServiceAccount)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create service_accounts table: %w", err)
	}

	// Create indexes for service_accounts
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_service_accounts_created_by ON service_accounts(created_by)`)
	if err != nil {
		return fmt.Errorf("failed to create service_accounts created_by index: %w", err)
	}

	// Add FK constraint for service_accounts.created_by → users.id
	_, err = db.Exec(`
		ALTER TABLE service_accounts
		ADD CONSTRAINT fk_service_accounts_created_by
		FOREIGN KEY (created_by) REFERENCES users(id)
	`)
	if err != nil {
		return fmt.Errorf("failed to add service_accounts created_by FK: %w", err)
	}
	fmt.Println(" OK")

	// 3. Create roles table
	fmt.Print(" [up] creating roles table...")
	_, err = db.NewCreateTable().
		Model((*models.Role)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create roles table: %w", err)
	}
	fmt.Println(" OK")

	// 4. Create user_roles table
	fmt.Print(" [up] creating user_roles table...")
	_, err = db.NewCreateTable().
		Model((*models.UserRole)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create user_roles table: %w", err)
	}

	// Add check constraint: exactly one of user_id or service_account_id must be non-null
	var checkConstraintSQL string
	if IsPostgreSQL(db) {
		// PostgreSQL: Use ::int casting
		checkConstraintSQL = `
			ALTER TABLE user_roles
			ADD CONSTRAINT chk_user_roles_identity_type
			CHECK ((user_id IS NOT NULL)::int + (service_account_id IS NOT NULL)::int = 1)
		`
	} else {
		// SQLite: Use CASE WHEN for boolean to int conversion
		checkConstraintSQL = `
			ALTER TABLE user_roles
			ADD CONSTRAINT chk_user_roles_identity_type
			CHECK (
				(CASE WHEN user_id IS NOT NULL THEN 1 ELSE 0 END) +
				(CASE WHEN service_account_id IS NOT NULL THEN 1 ELSE 0 END) = 1
			)
		`
	}

	_, err = db.Exec(checkConstraintSQL)
	if err != nil {
		return fmt.Errorf("failed to add user_roles identity check: %w", err)
	}

	// Partial unique indexes
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_user_role
		ON user_roles (user_id, role_id)
		WHERE service_account_id IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_roles user index: %w", err)
	}

	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_service_account_role
		ON user_roles (service_account_id, role_id)
		WHERE user_id IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_roles service_account index: %w", err)
	}

	// Create regular indexes for FK lookups
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id)`)
	if err != nil {
		return fmt.Errorf("failed to create user_roles user_id index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_roles_service_account_id ON user_roles(service_account_id)`)
	if err != nil {
		return fmt.Errorf("failed to create user_roles service_account_id index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id)`)
	if err != nil {
		return fmt.Errorf("failed to create user_roles role_id index: %w", err)
	}

	// Add FK constraints
	_, err = db.Exec(`
		ALTER TABLE user_roles
		ADD CONSTRAINT fk_user_roles_user_id
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("failed to add user_roles user_id FK: %w", err)
	}

	_, err = db.Exec(`
		ALTER TABLE user_roles
		ADD CONSTRAINT fk_user_roles_service_account_id
		FOREIGN KEY (service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("failed to add user_roles service_account_id FK: %w", err)
	}

	_, err = db.Exec(`
		ALTER TABLE user_roles
		ADD CONSTRAINT fk_user_roles_role_id
		FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("failed to add user_roles role_id FK: %w", err)
	}

	_, err = db.Exec(`
		ALTER TABLE user_roles
		ADD CONSTRAINT fk_user_roles_assigned_by
		FOREIGN KEY (assigned_by) REFERENCES users(id)
	`)
	if err != nil {
		return fmt.Errorf("failed to add user_roles assigned_by FK: %w", err)
	}
	fmt.Println(" OK")

	// 5. Create group_roles table
	fmt.Print(" [up] creating group_roles table...")
	_, err = db.NewCreateTable().
		Model((*models.GroupRole)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create group_roles table: %w", err)
	}

	// Create unique index on (group_name, role_id)
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_group_roles_unique
		ON group_roles (group_name, role_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create group_roles unique index: %w", err)
	}

	// Create indexes for lookups
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_group_roles_group_name ON group_roles(group_name)`)
	if err != nil {
		return fmt.Errorf("failed to create group_roles group_name index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_group_roles_role_id ON group_roles(role_id)`)
	if err != nil {
		return fmt.Errorf("failed to create group_roles role_id index: %w", err)
	}

	// Add FK constraints
	_, err = db.Exec(`
		ALTER TABLE group_roles
		ADD CONSTRAINT fk_group_roles_role_id
		FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("failed to add group_roles role_id FK: %w", err)
	}

	_, err = db.Exec(`
		ALTER TABLE group_roles
		ADD CONSTRAINT fk_group_roles_assigned_by
		FOREIGN KEY (assigned_by) REFERENCES users(id)
	`)
	if err != nil {
		return fmt.Errorf("failed to add group_roles assigned_by FK: %w", err)
	}
	fmt.Println(" OK")

	// 6. Create sessions table
	fmt.Print(" [up] creating sessions table...")
	_, err = db.NewCreateTable().
		Model((*models.Session)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create sessions table: %w", err)
	}

	// Add check constraint: exactly one of user_id or service_account_id must be non-null
	if IsPostgreSQL(db) {
		// PostgreSQL: Use ::int casting
		checkConstraintSQL = `
			ALTER TABLE sessions
			ADD CONSTRAINT chk_sessions_identity_type
			CHECK ((user_id IS NOT NULL)::int + (service_account_id IS NOT NULL)::int = 1)
		`
	} else {
		// SQLite: Use CASE WHEN for boolean to int conversion
		checkConstraintSQL = `
			ALTER TABLE sessions
			ADD CONSTRAINT chk_sessions_identity_type
			CHECK (
				(CASE WHEN user_id IS NOT NULL THEN 1 ELSE 0 END) +
				(CASE WHEN service_account_id IS NOT NULL THEN 1 ELSE 0 END) = 1
			)
		`
	}

	_, err = db.Exec(checkConstraintSQL)
	if err != nil {
		return fmt.Errorf("failed to add sessions identity check: %w", err)
	}

	// Create indexes
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`)
	if err != nil {
		return fmt.Errorf("failed to create sessions user_id index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_service_account_id ON sessions(service_account_id)`)
	if err != nil {
		return fmt.Errorf("failed to create sessions service_account_id index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`)
	if err != nil {
		return fmt.Errorf("failed to create sessions expires_at index: %w", err)
	}

	// Add FK constraints
	_, err = db.Exec(`
		ALTER TABLE sessions
		ADD CONSTRAINT fk_sessions_user_id
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("failed to add sessions user_id FK: %w", err)
	}

	_, err = db.Exec(`
		ALTER TABLE sessions
		ADD CONSTRAINT fk_sessions_service_account_id
		FOREIGN KEY (service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("failed to add sessions service_account_id FK: %w", err)
	}
	fmt.Println(" OK")

	// 7. Create casbin_rules table
	fmt.Print(" [up] creating casbin_rules table...")
	_, err = db.NewCreateTable().
		Model((*casbinbunadapter.CasbinRule)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create casbin_rules table: %w", err)
	}
	fmt.Println(" OK")

	return nil
}

// down_20251013140500 drops all auth tables in reverse order
func down_20251013140500(ctx context.Context, db *bun.DB) error {
	tables := []string{
		"casbin_rules",
		"sessions",
		"group_roles",
		"user_roles",
		"roles",
		"service_accounts",
		"users",
	}

	for _, table := range tables {
		fmt.Printf(" [down] dropping %s table...", table)
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to drop %s table: %w", table, err)
		}
		fmt.Println(" OK")
	}

	return nil
}
