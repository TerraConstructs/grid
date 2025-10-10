package migrations

import (
	"context"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251009000001, down_20251009000001)
}

// up_20251009000001 adds labels column to states and creates label_policy table
func up_20251009000001(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] adding labels column to states...")

	// 1. Add labels column to states table (PostgreSQL)
	_, err := db.Exec(`ALTER TABLE states ADD COLUMN IF NOT EXISTS labels JSONB NOT NULL DEFAULT '{}'::jsonb`)
	if err != nil {
		return fmt.Errorf("failed to add labels column: %w", err)
	}

	fmt.Println(" OK")
	fmt.Print(" [up] creating label_policy table...")

	// 2. Create label_policy table
	_, err = db.NewCreateTable().
		Model((*models.LabelPolicy)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create label_policy table: %w", err)
	}

	// Ensure policy_json column is JSONB
	_, err = db.Exec(`ALTER TABLE label_policy ALTER COLUMN policy_json TYPE JSONB USING policy_json::jsonb`)
	if err != nil {
		return fmt.Errorf("failed to ensure policy_json column is jsonb: %w", err)
	}

	// 3. Add single-row constraint (PostgreSQL)
	_, err = db.Exec(`
		ALTER TABLE label_policy
		ADD CONSTRAINT label_policy_single_row CHECK (id = 1)
	`)
	if err != nil {
		return fmt.Errorf("failed to add single-row constraint: %w", err)
	}

	// 4. Optional: Create GIN index for future SQL push-down optimization
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_states_labels_gin ON states USING gin (labels jsonb_path_ops)`)
	if err != nil {
		return fmt.Errorf("failed to create GIN index on labels: %w", err)
	}

	fmt.Println(" OK")
	return nil
}

// down_20251009000001 removes labels column and drops label_policy table
func down_20251009000001(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] dropping label_policy table...")

	_, err := db.NewDropTable().
		Model((*models.LabelPolicy)(nil)).
		IfExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop label_policy table: %w", err)
	}

	fmt.Println(" OK")
	fmt.Print(" [down] removing labels column from states...")

	_, err = db.Exec(`ALTER TABLE states DROP COLUMN IF EXISTS labels`)
	if err != nil {
		return fmt.Errorf("failed to drop labels column: %w", err)
	}

	fmt.Println(" OK")
	return nil
}
