package migrations

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	casbinbunadapter "github.com/terraconstructs/grid/cmd/gridapi/internal/auth/bunadapter"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251203000000, down_20251203000000)
}

// up_20251203000000 initializes the full database schema
func up_20251203000000(ctx context.Context, db *bun.DB) error {
	// 1. Create states table
	fmt.Print(" [up] creating states table...")
	_, err := db.NewCreateTable().
		Model((*models.State)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create states table: %w", err)
	}

	// Create index on labels
	if IsPostgreSQL(db) {
		// Use GIN index for JSONB
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_states_labels_gin ON states USING gin (labels jsonb_path_ops)`)
		if err != nil {
			return fmt.Errorf("failed to create GIN index on labels: %w", err)
		}
	} else {
		// Standard index for SQLite
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_states_labels ON states(labels)`)
		if err != nil {
			return fmt.Errorf("failed to create index on labels: %w", err)
		}
	}
	fmt.Println(" OK")

	// 2. Create label_policy table
	fmt.Print(" [up] creating label_policy table...")
	_, err = db.NewCreateTable().
		Model((*models.LabelPolicy)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create label_policy table: %w", err)
	}

	// Ensure policy_json is JSONB for Postgres
	if IsPostgreSQL(db) {
		_, err = db.Exec(`ALTER TABLE label_policy ALTER COLUMN policy_json TYPE JSONB USING policy_json::jsonb`)
		if err != nil {
			return fmt.Errorf("failed to ensure policy_json column is jsonb: %w", err)
		}
	}

	// Add single-row constraint (PostgreSQL only)
	// SQLite does not support ADD CONSTRAINT in ALTER TABLE.
	// For SQLite, this would need to be defined in CREATE TABLE, which Bun handles via tags or we skip for now.
	if IsPostgreSQL(db) {
		_, err = db.Exec(`
			ALTER TABLE label_policy
			ADD CONSTRAINT label_policy_single_row CHECK (id = 1)
		`)
		if err != nil {
			return fmt.Errorf("failed to add single-row constraint: %w", err)
		}
	}
	fmt.Println(" OK")

	// 3. Create edges table
	fmt.Print(" [up] creating edges table...")
	q := db.NewCreateTable().
		Model((*models.Edge)(nil)).
		IfNotExists()

	// For SQLite, define FKs during table creation
	if IsSQLite(db) {
		q = q.ForeignKey(`(from_state) REFERENCES states(guid) ON DELETE CASCADE`)
		q = q.ForeignKey(`(to_state) REFERENCES states(guid) ON DELETE CASCADE`)
	}
	_, err = q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create edges table: %w", err)
	}

	// Create edges indexes
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_edges_unique_from_output
		ON edges(from_state, from_output, to_state)
	`)
	if err != nil {
		return fmt.Errorf("failed to create unique index on (from_state, from_output, to_state): %w", err)
	}
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_edges_unique_to_input
		ON edges(to_state, to_input_name)
	`)
	if err != nil {
		return fmt.Errorf("failed to create unique index on (to_state, to_input_name): %w", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_edges_from_state ON edges(from_state)`)
	if err != nil {
		return fmt.Errorf("failed to create index on from_state: %w", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_edges_to_state ON edges(to_state)`)
	if err != nil {
		return fmt.Errorf("failed to create index on to_state: %w", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_edges_status ON edges(status)`)
	if err != nil {
		return fmt.Errorf("failed to create index on status: %w", err)
	}

	if IsPostgreSQL(db) {
		// Cycle prevention trigger (PG only)
		_, err = db.Exec(`
			CREATE OR REPLACE FUNCTION prevent_cycle() RETURNS trigger AS $$
			BEGIN
				IF EXISTS (
					WITH RECURSIVE reachable(node) AS (
						SELECT NEW.to_state
						UNION ALL
						SELECT e.to_state
						FROM edges e
						JOIN reachable r ON e.from_state = r.node
					)
					SELECT 1 FROM reachable WHERE node = NEW.from_state
				) THEN
					RAISE EXCEPTION 'cycle detected: % -> %', NEW.from_state, NEW.to_state;
				END IF;
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;
		`)
		if err != nil {
			return fmt.Errorf("failed to create prevent_cycle function: %w", err)
		}
		_, err = db.Exec(`
			CREATE TRIGGER edges_prevent_cycle
			BEFORE INSERT OR UPDATE ON edges
			FOR EACH ROW EXECUTE FUNCTION prevent_cycle();
		`)
		if err != nil {
			return fmt.Errorf("failed to create edges_prevent_cycle trigger: %w", err)
		}

		// FK constraints for PG
		_, err = db.Exec(`
			ALTER TABLE edges
			ADD CONSTRAINT fk_edges_from_state
			FOREIGN KEY (from_state) REFERENCES states(guid) ON DELETE CASCADE
		`)
		if err != nil {
			return fmt.Errorf("failed to add FK constraint on from_state: %w", err)
		}
		_, err = db.Exec(`
			ALTER TABLE edges
			ADD CONSTRAINT fk_edges_to_state
			FOREIGN KEY (to_state) REFERENCES states(guid) ON DELETE CASCADE
		`)
		if err != nil {
			return fmt.Errorf("failed to add FK constraint on to_state: %w", err)
		}
	}
	fmt.Println(" OK")

	// 4. Create state_outputs table
	fmt.Print(" [up] creating state_outputs table...")
	q = db.NewCreateTable().
		Model((*models.StateOutput)(nil)).
		IfNotExists()

	if IsSQLite(db) {
		q = q.ForeignKey(`(state_guid) REFERENCES states(guid) ON DELETE CASCADE`)
	}
	_, err = q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create state_outputs table: %w", err)
	}

	// Indexes and Constraints
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_state_outputs_guid ON state_outputs(state_guid)`)
	if err != nil {
		return fmt.Errorf("failed to create index on state_guid: %w", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_state_outputs_key ON state_outputs(output_key)`)
	if err != nil {
		return fmt.Errorf("failed to create index on output_key: %w", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_state_outputs_validation_status ON state_outputs(state_guid) WHERE validation_status IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("failed to create validation_status index: %w", err)
	}

	if IsPostgreSQL(db) {
		_, err = db.Exec(`
			ALTER TABLE state_outputs
			ADD CONSTRAINT fk_state_outputs_state_guid
			FOREIGN KEY (state_guid) REFERENCES states(guid) ON DELETE CASCADE
		`)
		if err != nil {
			return fmt.Errorf("failed to add FK constraint on state_guid: %w", err)
		}
	}
	fmt.Println(" OK")

	// 5. Auth Tables
	fmt.Print(" [up] creating auth tables...")

	// Users
	_, err = db.NewCreateTable().Model((*models.User)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return fmt.Errorf("create users: %w", err)
	}
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email)`)

	// Service Accounts
	q = db.NewCreateTable().Model((*models.ServiceAccount)(nil)).IfNotExists()
	if IsSQLite(db) {
		q = q.ForeignKey(`(created_by) REFERENCES users(id)`)
	}
	_, err = q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create service_accounts: %w", err)
	}
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_service_accounts_created_by ON service_accounts(created_by)`)
	if IsPostgreSQL(db) {
		db.Exec(`ALTER TABLE service_accounts ADD CONSTRAINT fk_service_accounts_created_by FOREIGN KEY (created_by) REFERENCES users(id)`)
	}

	// Roles
	_, err = db.NewCreateTable().Model((*models.Role)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return fmt.Errorf("create roles: %w", err)
	}

	// User Roles
	q = db.NewCreateTable().Model((*models.UserRole)(nil)).IfNotExists()
	if IsSQLite(db) {
		q = q.ForeignKey(`(user_id) REFERENCES users(id) ON DELETE CASCADE`)
		q = q.ForeignKey(`(service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE`)
		q = q.ForeignKey(`(role_id) REFERENCES roles(id) ON DELETE CASCADE`)
		q = q.ForeignKey(`(assigned_by) REFERENCES users(id)`)
	}
	_, err = q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create user_roles: %w", err)
	}

	// User Roles Constraints
	if IsPostgreSQL(db) {
		checkIdentity := `ALTER TABLE user_roles ADD CONSTRAINT chk_user_roles_identity_type CHECK ((user_id IS NOT NULL)::int + (service_account_id IS NOT NULL)::int = 1)`
		if _, err := db.Exec(checkIdentity); err != nil {
			return fmt.Errorf("user_roles constraint: %w", err)
		}
	}

	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_user_role ON user_roles (user_id, role_id) WHERE service_account_id IS NULL`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_service_account_role ON user_roles (service_account_id, role_id) WHERE user_id IS NULL`)

	if IsPostgreSQL(db) {
		db.Exec(`ALTER TABLE user_roles ADD CONSTRAINT fk_user_roles_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`)
		db.Exec(`ALTER TABLE user_roles ADD CONSTRAINT fk_user_roles_service_account_id FOREIGN KEY (service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE`)
		db.Exec(`ALTER TABLE user_roles ADD CONSTRAINT fk_user_roles_role_id FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE`)
		db.Exec(`ALTER TABLE user_roles ADD CONSTRAINT fk_user_roles_assigned_by FOREIGN KEY (assigned_by) REFERENCES users(id)`)
	}

	// Group Roles
	q = db.NewCreateTable().Model((*models.GroupRole)(nil)).IfNotExists()
	if IsSQLite(db) {
		q = q.ForeignKey(`(role_id) REFERENCES roles(id) ON DELETE CASCADE`)
		q = q.ForeignKey(`(assigned_by) REFERENCES users(id)`)
	}
	_, err = q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create group_roles: %w", err)
	}

	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_group_roles_unique ON group_roles (group_name, role_id)`)

	if IsPostgreSQL(db) {
		db.Exec(`ALTER TABLE group_roles ADD CONSTRAINT fk_group_roles_role_id FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE`)
		db.Exec(`ALTER TABLE group_roles ADD CONSTRAINT fk_group_roles_assigned_by FOREIGN KEY (assigned_by) REFERENCES users(id)`)
	}

	// Sessions
	q = db.NewCreateTable().Model((*models.Session)(nil)).IfNotExists()
	if IsSQLite(db) {
		q = q.ForeignKey(`(user_id) REFERENCES users(id) ON DELETE CASCADE`)
		q = q.ForeignKey(`(service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE`)
	}
	_, err = q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create sessions: %w", err)
	}

	if IsPostgreSQL(db) {
		checkSessionIdentity := `ALTER TABLE sessions ADD CONSTRAINT chk_sessions_identity_type CHECK ((user_id IS NOT NULL)::int + (service_account_id IS NOT NULL)::int = 1)`
		if _, err := db.Exec(checkSessionIdentity); err != nil {
			return fmt.Errorf("sessions constraint: %w", err)
		}
	}

	if IsPostgreSQL(db) {
		db.Exec(`ALTER TABLE sessions ADD CONSTRAINT fk_sessions_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`)
		db.Exec(`ALTER TABLE sessions ADD CONSTRAINT fk_sessions_service_account_id FOREIGN KEY (service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE`)
	}

	// Casbin Rules
	_, err = db.NewCreateTable().Model((*casbinbunadapter.CasbinRule)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return fmt.Errorf("create casbin_rules: %w", err)
	}

	// Revoked JTI
	_, err = db.NewCreateTable().Model((*models.RevokedJTI)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return fmt.Errorf("create revoked_jti: %w", err)
	}
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_revoked_jti_exp ON revoked_jti(exp)`)

	fmt.Println(" OK")

	// 6. Seed Data
	fmt.Print(" [up] seeding default data...")

	// Seed Roles
	defaultRoles := []models.Role{
		{Name: "service-account", Description: "Automation/CI-CD pipeline access (Data Plane only)", ScopeExpr: ""},
		{Name: "platform-engineer", Description: "Full admin access (Control + Data Plane)", ScopeExpr: ""},
		{Name: "product-engineer", Description: "Label-scoped access for product teams (dev environment)", ScopeExpr: `env == "dev"`, CreateConstraints: models.CreateConstraints{"env": {AllowedValues: []string{"dev"}, Required: true}}, ImmutableKeys: []string{"env"}},
	}
	for _, role := range defaultRoles {
		role.ID = uuid.Must(uuid.NewV7()).String()
		if _, err := db.NewInsert().Model(&role).On("CONFLICT (name) DO NOTHING").Exec(ctx); err != nil {
			return fmt.Errorf("seed role %s: %w", role.Name, err)
		}
	}

	// Seed System User
	sysSub := "system"
	sysUser := models.User{ID: auth.SystemUserID, Subject: &sysSub, Email: "system@grid.internal", Name: "System"}
	if _, err := db.NewInsert().Model(&sysUser).On("CONFLICT (id) DO NOTHING").Exec(ctx); err != nil {
		return fmt.Errorf("seed system user: %w", err)
	}

	// Seed Casbin Policies
	defaultPolicies := []casbinbunadapter.CasbinRule{
		// service-account role: Data plane access only (no scope constraint)
		{Ptype: "p", V0: "role:service-account", V1: "state", V2: "tfstate:read", V4: "allow"},
		{Ptype: "p", V0: "role:service-account", V1: "state", V2: "tfstate:write", V4: "allow"},
		{Ptype: "p", V0: "role:service-account", V1: "state", V2: "tfstate:lock", V4: "allow"},
		{Ptype: "p", V0: "role:service-account", V1: "state", V2: "tfstate:unlock", V4: "allow"},

		// platform-engineer role: Full access (wildcard, no constraint
		{Ptype: "p", V0: "role:platform-engineer", V1: "*", V2: "*", V4: "allow"},

		// product-engineer role: Label-scoped dev access
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:create", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:list", V4: "allow"}, // Listing allowed globally - service layer filters by role scopes
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:update-labels", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "tfstate:read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "tfstate:write", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "tfstate:lock", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "tfstate:unlock", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:list", V3: `env == "dev"`, V4: "allow"}, // List dependencies of a specific state (state-scoped)
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:list-all", V4: "allow"},                 // List all edges globally (filtered by handler)
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:create", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:delete", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state-output:list", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state-output:read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state-output:schema-write", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state-output:schema-read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "policy", V2: "policy:read", V4: "allow"},
	}
	if _, err := db.NewInsert().Model(&defaultPolicies).On("CONFLICT (ptype, v0, v1, v2, v3, v4, v5) DO NOTHING").Exec(ctx); err != nil {
		return fmt.Errorf("seed casbin policies: %w", err)
	}
	fmt.Println(" OK")

	return nil
}

// down_20251203000000 drops all tables
func down_20251203000000(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] dropping all tables...")

	// Drop triggers/functions first (PG)
	if IsPostgreSQL(db) {
		db.Exec("DROP TRIGGER IF EXISTS edges_prevent_cycle ON edges")
		db.Exec("DROP FUNCTION IF EXISTS prevent_cycle()")
	}

	tables := []string{
		"revoked_jti",
		"casbin_rules",
		"sessions",
		"group_roles",
		"user_roles",
		"roles",
		"service_accounts",
		"users",
		"state_outputs",
		"edges",
		"label_policy",
		"states",
	}

	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to drop %s: %w", table, err)
		}
	}

	fmt.Println(" OK")
	return nil
}
