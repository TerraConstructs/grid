package migrations

import "github.com/uptrace/bun"

// GetDialectName returns the database dialect name
func GetDialectName(db *bun.DB) string {
	return db.Dialect().Name()
}

// IsSQLite checks if the database is SQLite
func IsSQLite(db *bun.DB) bool {
	return GetDialectName(db) == "sqlite"
}

// IsPostgreSQL checks if the database is PostgreSQL
func IsPostgreSQL(db *bun.DB) bool {
	return GetDialectName(db) == "pg"
}
