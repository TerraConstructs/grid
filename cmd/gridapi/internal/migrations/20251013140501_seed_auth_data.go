package migrations

import (
	"context"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	casbinbunadapter "github.com/terraconstructs/grid/cmd/gridapi/internal/auth/bunadapter"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251013140501, down_20251013140501)
}

// up_20251013140501 seeds default roles and Casbin policies
func up_20251013140501(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] seeding default roles...")

	// 1. Seed default roles
	defaultRoles := []models.Role{
		{
			Name:        "service-account",
			Description: "Automation/CI-CD pipeline access (Data Plane only)",
			ScopeExpr:   "", // No scope constraint
		},
		{
			Name:        "platform-engineer",
			Description: "Full admin access (Control + Data Plane)",
			ScopeExpr:   "", // No scope constraint
		},
		{
			Name:        "product-engineer",
			Description: "Label-scoped access for product teams (dev environment)",
			ScopeExpr:   `env == "dev"`,
			CreateConstraints: models.CreateConstraints{
				"env": {
					AllowedValues: []string{"dev"},
					Required:      true,
				},
			},
			ImmutableKeys: []string{"env"},
		},
	}

	for _, role := range defaultRoles {
		_, err := db.NewInsert().
			Model(&role).
			On("CONFLICT (name) DO NOTHING"). // Idempotent
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to seed role %s: %w", role.Name, err)
		}
	}
	fmt.Println(" OK")

	fmt.Print(" [up] seeding system user...")

	// 1.5. Create system user for seed data attribution
	systemSubject := "system"
	systemUser := models.User{
		ID:      auth.SystemUserID,
		Subject: &systemSubject,
		Email:   "system@grid.internal",
		Name:    "System",
	}

	_, err := db.NewInsert().
		Model(&systemUser).
		On("CONFLICT (id) DO NOTHING"). // Idempotent
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to seed system user: %w", err)
	}
	fmt.Println(" OK")

	fmt.Print(" [up] seeding default Casbin policies...")

	// 2. Seed default Casbin policies
	// Using model: p = role, objType, act, scopeExpr, eft
	defaultPolicies := []casbinbunadapter.CasbinRule{
		// service-account role: Data plane access only (no scope constraint)
		{Ptype: "p", V0: "role:service-account", V1: "state", V2: "tfstate:read", V3: "", V4: "allow"},
		{Ptype: "p", V0: "role:service-account", V1: "state", V2: "tfstate:write", V3: "", V4: "allow"},
		{Ptype: "p", V0: "role:service-account", V1: "state", V2: "tfstate:lock", V3: "", V4: "allow"},
		{Ptype: "p", V0: "role:service-account", V1: "state", V2: "tfstate:unlock", V3: "", V4: "allow"},

		// platform-engineer role: Full access (wildcard, no constraint)
		{Ptype: "p", V0: "role:platform-engineer", V1: "*", V2: "*", V3: "", V4: "allow"},

		// product-engineer role: Label-scoped dev access
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:create", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:list", V3: "", V4: "allow"}, // Listing allowed globally - service layer filters by role scopes
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:update-labels", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "tfstate:read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "tfstate:write", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "tfstate:lock", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "tfstate:unlock", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:list", V3: `env == "dev"`, V4: "allow"},     // List dependencies of a specific state (state-scoped)
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:list-all", V3: "", V4: "allow"},           // List all edges globally (filtered by handler)
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:create", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "dependency:delete", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state-output:list", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state-output:read", V3: `env == "dev"`, V4: "allow"},
		{Ptype: "p", V0: "role:product-engineer", V1: "policy", V2: "policy:read", V3: "", V4: "allow"},
	}

	// Bulk insert with conflict handling
	_, err = db.NewInsert().
		Model(&defaultPolicies).
		On("CONFLICT (ptype, v0, v1, v2, v3, v4, v5) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to seed Casbin policies: %w", err)
	}
	fmt.Println(" OK")

	return nil
}

// down_20251013140501 removes seeded data
func down_20251013140501(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] removing seeded Casbin policies...")

	// Remove seeded policies
	_, err := db.NewDelete().
		Model((*casbinbunadapter.CasbinRule)(nil)).
		Where("v0 IN (?)", bun.In([]string{"role:service-account", "role:platform-engineer", "role:product-engineer"})).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove seeded policies: %w", err)
	}
	fmt.Println(" OK")

	fmt.Print(" [down] removing gridctl public client...")

	// Remove gridctl client
	_, err = db.NewDelete().
		Model((*models.ServiceAccount)(nil)).
		Where("client_id = ?", "gridctl").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove gridctl client: %w", err)
	}
	fmt.Println(" OK")

	fmt.Print(" [down] removing system user...")

	// Remove system user
	_, err = db.NewDelete().
		Model((*models.User)(nil)).
		Where("id = ?", "00000000-0000-0000-0000-000000000000").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove system user: %w", err)
	}
	fmt.Println(" OK")

	fmt.Print(" [down] removing seeded roles...")

	// Remove seeded roles
	_, err = db.NewDelete().
		Model((*models.Role)(nil)).
		Where("name IN (?)", bun.In([]string{"service-account", "platform-engineer", "product-engineer"})).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove seeded roles: %w", err)
	}
	fmt.Println(" OK")

	return nil
}
