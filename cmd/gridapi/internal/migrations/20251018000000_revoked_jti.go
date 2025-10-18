package migrations

import (
	"context"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251018000000, down_20251018000000)
}

// up_20251018000000 creates revoked_jti table for JWT token revocation
func up_20251018000000(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] creating revoked_jti table...")

	_, err := db.NewCreateTable().
		Model((*models.RevokedJTI)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create revoked_jti table: %w", err)
	}

	// Create index on exp for cleanup queries
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_revoked_jti_exp ON revoked_jti(exp)
	`)
	if err != nil {
		return fmt.Errorf("failed to create revoked_jti exp index: %w", err)
	}
	fmt.Println(" OK")

	return nil
}

// down_20251018000000 drops revoked_jti table
func down_20251018000000(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] dropping revoked_jti table...")

	_, err := db.NewDropTable().
		Model((*models.RevokedJTI)(nil)).
		IfExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to drop revoked_jti table: %w", err)
	}
	fmt.Println(" OK")

	return nil
}
