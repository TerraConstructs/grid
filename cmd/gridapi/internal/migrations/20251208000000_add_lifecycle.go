package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251208000000, down_20251208000000)
}

// up_20251208000000 adds lifecycle columns to states table
func up_20251208000000(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] adding lifecycle columns to states table...")

	var err error

	// Add status column (IF NOT EXISTS for PostgreSQL, idempotent for fresh DBs)
	if IsPostgreSQL(db) {
		_, err = db.Exec(`ALTER TABLE states ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'active'`)
		if err != nil {
			return fmt.Errorf("add status column: %w", err)
		}
	} else {
		// SQLite doesn't support IF NOT EXISTS in ALTER TABLE, check column existence first
		var exists bool
		err = db.NewSelect().
			ColumnExpr("COUNT(*) > 0").
			TableExpr("pragma_table_info('states')").
			Where("name = ?", "status").
			Scan(ctx, &exists)
		if err == nil && !exists {
			_, err = db.Exec(`ALTER TABLE states ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active'`)
			if err != nil {
				return fmt.Errorf("add status column: %w", err)
			}
		}
	}

	// Add tombstoned_at column
	if IsPostgreSQL(db) {
		_, err = db.Exec(`ALTER TABLE states ADD COLUMN IF NOT EXISTS tombstoned_at TIMESTAMP NULL`)
		if err != nil {
			return fmt.Errorf("add tombstoned_at column: %w", err)
		}
	} else {
		var exists bool
		err = db.NewSelect().
			ColumnExpr("COUNT(*) > 0").
			TableExpr("pragma_table_info('states')").
			Where("name = ?", "tombstoned_at").
			Scan(ctx, &exists)
		if err == nil && !exists {
			_, err = db.Exec(`ALTER TABLE states ADD COLUMN tombstoned_at TIMESTAMP NULL`)
			if err != nil {
				return fmt.Errorf("add tombstoned_at column: %w", err)
			}
		}
	}

	// Add tombstoned_by column
	if IsPostgreSQL(db) {
		_, err = db.Exec(`ALTER TABLE states ADD COLUMN IF NOT EXISTS tombstoned_by VARCHAR(255) NULL`)
		if err != nil {
			return fmt.Errorf("add tombstoned_by column: %w", err)
		}
	} else {
		var exists bool
		err = db.NewSelect().
			ColumnExpr("COUNT(*) > 0").
			TableExpr("pragma_table_info('states')").
			Where("name = ?", "tombstoned_by").
			Scan(ctx, &exists)
		if err == nil && !exists {
			_, err = db.Exec(`ALTER TABLE states ADD COLUMN tombstoned_by VARCHAR(255) NULL`)
			if err != nil {
				return fmt.Errorf("add tombstoned_by column: %w", err)
			}
		}
	}

	// Add retention_days column
	if IsPostgreSQL(db) {
		_, err = db.Exec(`ALTER TABLE states ADD COLUMN IF NOT EXISTS retention_days INTEGER NOT NULL DEFAULT 30`)
		if err != nil {
			return fmt.Errorf("add retention_days column: %w", err)
		}
	} else {
		var exists bool
		err = db.NewSelect().
			ColumnExpr("COUNT(*) > 0").
			TableExpr("pragma_table_info('states')").
			Where("name = ?", "retention_days").
			Scan(ctx, &exists)
		if err == nil && !exists {
			_, err = db.Exec(`ALTER TABLE states ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 30`)
			if err != nil {
				return fmt.Errorf("add retention_days column: %w", err)
			}
		}
	}

	// Create index for efficient status filtering
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_states_status ON states(status)`)
	if err != nil {
		return fmt.Errorf("create status index: %w", err)
	}

	// Composite index for tombstone retention queries
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_states_tombstone ON states(status, tombstoned_at)`)
	if err != nil {
		return fmt.Errorf("create tombstone index: %w", err)
	}

	// CHECK constraint only for PostgreSQL
	if IsPostgreSQL(db) {
		_, err = db.Exec(`ALTER TABLE states ADD CONSTRAINT chk_states_status CHECK (status IN ('active', 'tombstoned'))`)
		if err != nil {
			return fmt.Errorf("add status check constraint: %w", err)
		}
	}
	// For SQLite: Status validation enforced in StateService and repository layer

	fmt.Println(" OK")
	return nil
}

// down_20251208000000 removes lifecycle columns from states table
func down_20251208000000(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] removing lifecycle columns from states table...")

	// Drop indexes first
	_, _ = db.Exec(`DROP INDEX IF EXISTS idx_states_tombstone`)
	_, _ = db.Exec(`DROP INDEX IF EXISTS idx_states_status`)

	// Drop constraint (PostgreSQL only)
	if IsPostgreSQL(db) {
		_, _ = db.Exec(`ALTER TABLE states DROP CONSTRAINT IF EXISTS chk_states_status`)
	}

	// Drop columns
	if IsPostgreSQL(db) {
		// PostgreSQL supports DROP COLUMN
		_, _ = db.Exec(`ALTER TABLE states DROP COLUMN IF EXISTS retention_days`)
		_, _ = db.Exec(`ALTER TABLE states DROP COLUMN IF EXISTS tombstoned_by`)
		_, _ = db.Exec(`ALTER TABLE states DROP COLUMN IF EXISTS tombstoned_at`)
		_, _ = db.Exec(`ALTER TABLE states DROP COLUMN IF EXISTS status`)
	} else {
		// SQLite doesn't support DROP COLUMN directly in older versions
		// For modern SQLite (3.35.0+), DROP COLUMN is supported
		_, _ = db.Exec(`ALTER TABLE states DROP COLUMN retention_days`)
		_, _ = db.Exec(`ALTER TABLE states DROP COLUMN tombstoned_by`)
		_, _ = db.Exec(`ALTER TABLE states DROP COLUMN tombstoned_at`)
		_, _ = db.Exec(`ALTER TABLE states DROP COLUMN status`)
	}

	fmt.Println(" OK")
	return nil
}
