package history

import "database/sql"

// DB exposes the process-wide durable SQL handle to internal domain stores that
// share Trendinary's persistence layer. Production uses Turso/libSQL; local
// development and tests may use SQLite. Callers must not close it.
func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}
