package bunx

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// NewDB creates a new Bun database instance connected to PostgreSQL
func NewDB(dsn string) (*bun.DB, error) {
	// Create pgdriver connector
	connector := pgdriver.NewConnector(pgdriver.WithDSN(dsn))

	// Create SQL DB with the connector
	sqldb := sql.OpenDB(connector)

	// Configure connection pool
	sqldb.SetMaxOpenConns(25)
	sqldb.SetMaxIdleConns(25)

	// Create Bun DB with PostgreSQL dialect
	db := bun.NewDB(sqldb, pgdialect.New())

	// Verify connectivity
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func Close(db *bun.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
