package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/migrations"
	"github.com/uptrace/bun/migrate"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long:  `Commands for managing database migrations and schema.`,
}

var dbInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize migration tables",
	Long:  `Creates the migration tracking tables in the database. Run this once during initial setup.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		migrator := migrate.NewMigrator(db, migrations.Migrations)

		ctx := context.Background()
		if err := migrator.Init(ctx); err != nil {
			return fmt.Errorf("failed to initialize migrator: %w", err)
		}

		log.Printf("Migration tables initialized successfully")
		return nil
	},
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Applies all pending migrations to the database with locking to prevent concurrent migrations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		migrator := migrate.NewMigrator(db, migrations.Migrations)

		ctx := context.Background()

		// Acquire lock to prevent concurrent migrations
		if err := migrator.Lock(ctx); err != nil {
			return fmt.Errorf("failed to acquire migration lock: %w", err)
		}
		defer func() {
			if err := migrator.Unlock(ctx); err != nil {
				log.Printf("Warning: failed to release migration lock: %v", err)
			}
		}()

		group, err := migrator.Migrate(ctx)
		if err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}

		if group.ID == 0 {
			log.Printf("No new migrations to apply")
		} else {
			log.Printf("Applied migration group %d", group.ID)
		}

		return nil
	},
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long:  `Displays the current migration status and pending migrations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		migrator := migrate.NewMigrator(db, migrations.Migrations)

		ctx := context.Background()
		ms, err := migrator.MigrationsWithStatus(ctx)
		if err != nil {
			return fmt.Errorf("failed to get migration status: %w", err)
		}

		log.Printf("Migrations:")
		for _, m := range ms {
			status := "pending"
			if m.GroupID > 0 {
				status = fmt.Sprintf("applied (group %d)", m.GroupID)
			}
			log.Printf("  %s: %s", m.Name, status)
		}

		return nil
	},
}

var dbRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback last migration group",
	Long:  `Rolls back the most recently applied migration group with locking to prevent concurrent operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		migrator := migrate.NewMigrator(db, migrations.Migrations)

		ctx := context.Background()

		// Acquire lock to prevent concurrent rollbacks
		if err := migrator.Lock(ctx); err != nil {
			return fmt.Errorf("failed to acquire migration lock: %w", err)
		}
		defer func() {
			if err := migrator.Unlock(ctx); err != nil {
				log.Printf("Warning: failed to release migration lock: %v", err)
			}
		}()

		group, err := migrator.Rollback(ctx)
		if err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}

		if group.ID == 0 {
			log.Printf("No migrations to rollback")
		} else {
			log.Printf("Rolled back migration group %d", group.ID)
		}

		return nil
	},
}

var dbLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Manually acquire migration lock",
	Long:  `Acquires the migration lock. Useful for debugging or maintenance operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		migrator := migrate.NewMigrator(db, migrations.Migrations)

		ctx := context.Background()
		if err := migrator.Lock(ctx); err != nil {
			return fmt.Errorf("failed to acquire migration lock: %w", err)
		}

		log.Printf("Migration lock acquired successfully")
		log.Printf("Remember to run 'db unlock' when finished")
		return nil
	},
}

var dbUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Force release migration lock",
	Long:  `Force releases the migration lock. Use this if a migration crashed while holding the lock.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		migrator := migrate.NewMigrator(db, migrations.Migrations)

		ctx := context.Background()
		if err := migrator.Unlock(ctx); err != nil {
			return fmt.Errorf("failed to release migration lock: %w", err)
		}

		log.Printf("Migration lock released successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbInitCmd)
	dbCmd.AddCommand(dbMigrateCmd)
	dbCmd.AddCommand(dbStatusCmd)
	dbCmd.AddCommand(dbRollbackCmd)
	dbCmd.AddCommand(dbLockCmd)
	dbCmd.AddCommand(dbUnlockCmd)
}
