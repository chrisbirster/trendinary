package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

const cursorSchema = `
CREATE TABLE IF NOT EXISTS stream_cursors (
  name TEXT PRIMARY KEY,
  seq INTEGER NOT NULL,
  updated_at TEXT NOT NULL
);`

// A Trendinary process normally owns one history Store. Tests may create more,
// so schema readiness is tracked per Store rather than with a package-wide Once.
// This avoids executing DDL for every high-volume Jetstream cursor checkpoint.
var cursorSchemaReady sync.Map // map[*Store]struct{}

func (s *Store) ensureCursorSchema(ctx context.Context) error {
	if _, ok := cursorSchemaReady.Load(s); ok {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, cursorSchema); err != nil {
		return fmt.Errorf("ensure cursor schema: %w", err)
	}
	cursorSchemaReady.Store(s, struct{}{})
	return nil
}

// Cursor returns the durable resume cursor for a named stream. Stream cursors
// are intentionally independent from trend snapshots because a consumer must
// only advance its cursor after the corresponding event batch has been folded
// into durable application state.
func (s *Store) Cursor(ctx context.Context, name string) (uint64, bool, error) {
	if name == "" {
		return 0, false, fmt.Errorf("cursor name is required")
	}
	if err := s.ensureCursorSchema(ctx); err != nil {
		return 0, false, err
	}

	var seq int64
	err := s.db.QueryRowContext(ctx, `SELECT seq FROM stream_cursors WHERE name = ?`, name).Scan(&seq)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read cursor %q: %w", name, err)
	}
	if seq < 0 {
		return 0, false, fmt.Errorf("cursor %q contains invalid negative sequence", name)
	}
	return uint64(seq), true, nil
}

// SaveCursor commits a stream cursor after a batch has been processed. SQLite
// INTEGER is signed, so reject a sequence that cannot be represented rather
// than silently wrapping it.
func (s *Store) SaveCursor(ctx context.Context, name string, seq uint64) error {
	if name == "" {
		return fmt.Errorf("cursor name is required")
	}
	if seq > math.MaxInt64 {
		return fmt.Errorf("cursor %q sequence %d exceeds SQLite INTEGER", name, seq)
	}
	if err := s.ensureCursorSchema(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO stream_cursors (name, seq, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
  seq=excluded.seq,
  updated_at=excluded.updated_at`,
		name,
		int64(seq),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save cursor %q: %w", name, err)
	}
	return nil
}
