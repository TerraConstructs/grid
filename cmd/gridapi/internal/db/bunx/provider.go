package bunx

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	_ "modernc.org/sqlite" // SQLite driver
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgres"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
)

// DetectDatabaseType determines the database type from a DSN string
func DetectDatabaseType(dsn string) DatabaseType {
	// PostgreSQL DSN patterns
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return DatabaseTypePostgreSQL
	}
	// SQLite patterns: file:, :memory:, or plain file path
	return DatabaseTypeSQLite
}

// NewDB creates a new Bun database instance for PostgreSQL or SQLite based on DSN
func NewDB(dsn string) (*bun.DB, error) {
	dbType := DetectDatabaseType(dsn)

	switch dbType {
	case DatabaseTypePostgreSQL:
		return newPostgreSQLDB(dsn)
	case DatabaseTypeSQLite:
		return newSQLiteDB(dsn)
	default:
		return nil, fmt.Errorf("unsupported database type for DSN: %s", dsn)
	}
}

// newPostgreSQLDB creates a PostgreSQL connection
func newPostgreSQLDB(dsn string) (*bun.DB, error) {
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

// newSQLiteDB creates a SQLite connection using modernc.org/sqlite driver
func newSQLiteDB(dsn string) (*bun.DB, error) {
	// Open SQLite database
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure connection pool based on database type
	isInMemory := dsn == ":memory:" || strings.Contains(dsn, "mode=memory")

	if isInMemory {
		// In-memory database: Keep connections alive to prevent database destruction
		// Critical: In-memory databases are destroyed when all connections close
		sqldb.SetMaxOpenConns(1)      // Single connection for consistency
		sqldb.SetMaxIdleConns(1)       // Keep the connection alive
		sqldb.SetConnMaxLifetime(0)    // No connection expiry
	} else {
		// File-based database: Standard connection pool
		sqldb.SetMaxOpenConns(1)       // Single writer for SQLite best practices
		sqldb.SetMaxIdleConns(2)       // Allow some idle connections
	}

	// Create Bun DB with SQLite dialect
	db := bun.NewDB(sqldb, sqlitedialect.New())

	// Enable foreign keys (disabled by default in SQLite)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Verify connectivity
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
