package migrations

import (
	"context"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251003140000, down_20251003140000)
}

// up_20251003140000 creates the state_outputs table for caching TF outputs
func up_20251003140000(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] creating state_outputs table...")

	// Create state_outputs table
	_, err := db.NewCreateTable().
		Model((*models.StateOutput)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create state_outputs table: %w", err)
	}

	// Create index on state_guid for bulk lookup
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_state_outputs_guid ON state_outputs(state_guid)`)
	if err != nil {
		return fmt.Errorf("failed to create index on state_guid: %w", err)
	}

	// Create index on output_key for cross-state search
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_state_outputs_key ON state_outputs(output_key)`)
	if err != nil {
		return fmt.Errorf("failed to create index on output_key: %w", err)
	}

	// Add foreign key constraint with cascade delete
	_, err = db.Exec(`
		ALTER TABLE state_outputs
		ADD CONSTRAINT fk_state_outputs_state_guid
		FOREIGN KEY (state_guid) REFERENCES states(guid) ON DELETE CASCADE
	`)
	if err != nil {
		return fmt.Errorf("failed to add FK constraint on state_guid: %w", err)
	}

	fmt.Println(" OK")
	return nil
}

// down_20251003140000 drops the state_outputs table
func down_20251003140000(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] dropping state_outputs table...")

	_, err := db.NewDropTable().
		Model((*models.StateOutput)(nil)).
		IfExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to drop state_outputs table: %w", err)
	}

	fmt.Println(" OK")
	return nil
}
