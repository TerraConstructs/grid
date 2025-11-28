package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251125000002, down_20251125000002)
}

// up_20251125000002 adds schema_source and validation columns to state_outputs table
func up_20251125000002(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] adding schema_source and validation columns to state_outputs...")

	// Add schema_source column
	_, err := db.Exec(`ALTER TABLE state_outputs ADD COLUMN IF NOT EXISTS schema_source TEXT CHECK (schema_source IN ('manual', 'inferred'))`)
	if err != nil {
		return fmt.Errorf("failed to add schema_source column: %w", err)
	}

	// Add validation_status column
	_, err = db.Exec(`ALTER TABLE state_outputs ADD COLUMN IF NOT EXISTS validation_status TEXT CHECK (validation_status IN ('valid', 'invalid', 'error'))`)
	if err != nil {
		return fmt.Errorf("failed to add validation_status column: %w", err)
	}

	// Add validation_error column
	_, err = db.Exec(`ALTER TABLE state_outputs ADD COLUMN IF NOT EXISTS validation_error TEXT`)
	if err != nil {
		return fmt.Errorf("failed to add validation_error column: %w", err)
	}

	// Add validated_at column
	_, err = db.Exec(`ALTER TABLE state_outputs ADD COLUMN IF NOT EXISTS validated_at TIMESTAMPTZ`)
	if err != nil {
		return fmt.Errorf("failed to add validated_at column: %w", err)
	}

	// Add index for validation status queries
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_state_outputs_validation_status ON state_outputs(state_guid) WHERE validation_status IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("failed to create validation_status index: %w", err)
	}

	fmt.Println(" OK")
	return nil
}

// down_20251125000002 removes schema_source and validation columns from state_outputs table
func down_20251125000002(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] removing schema_source and validation columns from state_outputs...")

	// Drop index first
	_, err := db.Exec(`DROP INDEX IF EXISTS idx_state_outputs_validation_status`)
	if err != nil {
		return fmt.Errorf("failed to drop validation_status index: %w", err)
	}

	// Drop columns
	_, err = db.Exec(`ALTER TABLE state_outputs DROP COLUMN IF EXISTS validated_at`)
	if err != nil {
		return fmt.Errorf("failed to drop validated_at column: %w", err)
	}

	_, err = db.Exec(`ALTER TABLE state_outputs DROP COLUMN IF EXISTS validation_error`)
	if err != nil {
		return fmt.Errorf("failed to drop validation_error column: %w", err)
	}

	_, err = db.Exec(`ALTER TABLE state_outputs DROP COLUMN IF EXISTS validation_status`)
	if err != nil {
		return fmt.Errorf("failed to drop validation_status column: %w", err)
	}

	_, err = db.Exec(`ALTER TABLE state_outputs DROP COLUMN IF EXISTS schema_source`)
	if err != nil {
		return fmt.Errorf("failed to drop schema_source column: %w", err)
	}

	fmt.Println(" OK")
	return nil
}
