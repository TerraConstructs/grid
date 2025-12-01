package migrations

import (
	"context"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251002000002, down_20251002000002)
}

// up_20251002000002 creates the edges table with cycle prevention trigger
func up_20251002000002(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] creating edges table...")

	// Create edges table
	_, err := db.NewCreateTable().
		Model((*models.Edge)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create edges table: %w", err)
	}

	// Add composite unique constraints
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

	// Create additional indexes for performance
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

	// Create cycle prevention trigger (PostgreSQL only)
	// SQLite relies on application-layer cycle check in EdgeRepository.WouldCreateCycle()
	if IsPostgreSQL(db) {
		// Create trigger function
		_, err = db.Exec(`
			CREATE OR REPLACE FUNCTION prevent_cycle() RETURNS trigger AS $$
			BEGIN
				-- Check if NEW.to_state can already reach NEW.from_state
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

		// Create trigger
		_, err = db.Exec(`
			CREATE TRIGGER edges_prevent_cycle
			BEFORE INSERT OR UPDATE ON edges
			FOR EACH ROW EXECUTE FUNCTION prevent_cycle();
		`)
		if err != nil {
			return fmt.Errorf("failed to create edges_prevent_cycle trigger: %w", err)
		}
	}

	// Add foreign key constraints with cascade delete
	// Note: SQLite requires foreign keys to be defined during table creation
	// or via ALTER TABLE ADD CONSTRAINT (supported since 3.25.0)
	if IsPostgreSQL(db) {
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
	// For SQLite: Foreign keys are defined in the model's bun tags and created during NewCreateTable()

	fmt.Println(" OK")
	return nil
}

// down_20251002000002 drops the edges table and related objects
func down_20251002000002(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] dropping edges table...")

	// Drop PostgreSQL-specific trigger and function
	if IsPostgreSQL(db) {
		// Drop trigger
		_, err := db.Exec("DROP TRIGGER IF EXISTS edges_prevent_cycle ON edges")
		if err != nil {
			return fmt.Errorf("failed to drop trigger: %w", err)
		}

		// Drop function
		_, err = db.Exec("DROP FUNCTION IF EXISTS prevent_cycle()")
		if err != nil {
			return fmt.Errorf("failed to drop function: %w", err)
		}
	}

	// Drop table (cascade will handle constraints)
	_, err := db.NewDropTable().
		Model((*models.Edge)(nil)).
		IfExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to drop edges table: %w", err)
	}

	fmt.Println(" OK")
	return nil
}
