package bunx

import (
	"testing"
)

func TestDetectDatabaseType(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected DatabaseType
	}{
		{
			name:     "postgres scheme",
			dsn:      "postgres://user:pass@localhost:5432/dbname",
			expected: DatabaseTypePostgreSQL,
		},
		{
			name:     "postgresql scheme",
			dsn:      "postgresql://user:pass@localhost:5432/dbname",
			expected: DatabaseTypePostgreSQL,
		},
		{
			name:     "unix socket scheme",
			dsn:      "unix://user:pass@dbname/var/run/postgresql/.s.PGSQL.5432",
			expected: DatabaseTypePostgreSQL,
		},
		{
			name:     "unix socket with query params",
			dsn:      "unix://grid:gridpass@grid/var/run/postgresql/.s.PGSQL.5432?sslmode=disable",
			expected: DatabaseTypePostgreSQL,
		},
		{
			name:     "sqlite in-memory",
			dsn:      ":memory:",
			expected: DatabaseTypeSQLite,
		},
		{
			name:     "sqlite file path",
			dsn:      "/path/to/database.db",
			expected: DatabaseTypeSQLite,
		},
		{
			name:     "sqlite file:// scheme",
			dsn:      "file:/path/to/database.db",
			expected: DatabaseTypeSQLite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectDatabaseType(tt.dsn)
			if result != tt.expected {
				t.Errorf("DetectDatabaseType(%q) = %v, expected %v", tt.dsn, result, tt.expected)
			}
		})
	}
}
