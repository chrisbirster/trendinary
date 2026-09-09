package history

import (
	"context"
	"fmt"
)

// Ready verifies that the durable database is reachable and that Atlas
// left the runtime-required schema in place. It is strictly read-only.
func (s *Store) Ready(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is unavailable")
	}
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	if err := s.VerifySchema(ctx); err != nil {
		return fmt.Errorf("database schema: %w", err)
	}
	return nil
}
