package history

import (
	"context"
	"fmt"
)

// DeleteSignals removes source observations that were deleted upstream. The
// operation is idempotent so replaying a Jetstream batch after a crash is safe.
func (s *Store) DeleteSignals(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin signal delete: %w", err)
	}
	defer tx.Rollback()

	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM signals WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete signal %s: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit signal delete: %w", err)
	}
	return nil
}
