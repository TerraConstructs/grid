package bunx

import "github.com/google/uuid"

// NewUUIDv7 generates a time-ordered UUIDv7 string for database primary keys.
//
// UUIDv7 provides:
//   - Time-ordered sortability for better database index performance
//   - Compatibility with both PostgreSQL and SQLite (no gen_random_uuid() dependency)
//   - Monotonic ordering within the same millisecond
//
// This function panics if UUID generation fails, which only occurs on catastrophic
// system failures (e.g., entropy source exhaustion). This is acceptable because:
//   - UUID generation failure means the system cannot operate safely
//   - All database ID generation would fail anyway
//   - Explicit error handling adds complexity without practical benefit
//
// Use this for all model ID fields instead of uuid.NewString() (UUIDv4).
func NewUUIDv7() string {
	return uuid.Must(uuid.NewV7()).String()
}
