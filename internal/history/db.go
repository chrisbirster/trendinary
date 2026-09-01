package history

import "database/sql"

// DB exposes the process-wide SQLite handle to internal domain stores that need
// to share the same writer/connection policy. Callers must not close it.
func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}
