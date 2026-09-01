package history

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Backup writes a transactionally consistent local SQLite snapshot using
// VACUUM INTO. Remote Turso databases use Turso's point-in-time recovery and
// export tooling instead of copying application-container state.
func (s *Store) Backup(ctx context.Context, destination string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("history store is not configured")
	}
	if s.Backend() == BackendTurso {
		return fmt.Errorf("application-level database backup is not supported for Turso; use Turso point-in-time recovery or database export")
	}
	if strings.TrimSpace(destination) == "" {
		return fmt.Errorf("backup destination is required")
	}
	_ = os.Remove(destination)
	escaped := strings.ReplaceAll(destination, "'", "''")
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil {
		return fmt.Errorf("sqlite backup: %w", err)
	}
	return nil
}
