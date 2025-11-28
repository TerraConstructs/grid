package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251123000001, down_20251123000001)
}

// up_20251123000001 adds schema_json column to state_outputs table
func up_20251123000001(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] adding schema_json column to state_outputs...")

	// Add schema_json column to state_outputs table (PostgreSQL)
	_, err := db.Exec(`ALTER TABLE state_outputs ADD COLUMN IF NOT EXISTS schema_json TEXT`)
	if err != nil {
		return fmt.Errorf("failed to add schema_json column: %w", err)
	}

	fmt.Println(" OK")
	return nil
}

// down_20251123000001 removes schema_json column from state_outputs table
func down_20251123000001(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] removing schema_json column from state_outputs...")

	_, err := db.Exec(`ALTER TABLE state_outputs DROP COLUMN IF EXISTS schema_json`)
	if err != nil {
		return fmt.Errorf("failed to drop schema_json column: %w", err)
	}

	fmt.Println(" OK")
	return nil
}
