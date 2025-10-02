package migrations

import (
	"context"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20250930000001, down_20250930000001)
}

// up_20250930000001 creates the states table
func up_20250930000001(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] creating states table...")

	_, err := db.NewCreateTable().
		Model((*models.State)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create states table: %w", err)
	}

	fmt.Println(" OK")
	return nil
}

// down_20250930000001 drops the states table
func down_20250930000001(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] dropping states table...")

	_, err := db.NewDropTable().
		Model((*models.State)(nil)).
		IfExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to drop states table: %w", err)
	}

	fmt.Println(" OK")
	return nil
}
