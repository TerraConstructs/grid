package migrations

import (
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

// IsSQLite checks if the database is SQLite
func IsSQLite(db *bun.DB) bool {
	return db.Dialect().Name() == dialect.SQLite
}

// IsPostgreSQL checks if the database is PostgreSQL
func IsPostgreSQL(db *bun.DB) bool {
	return db.Dialect().Name() == dialect.PG
}
