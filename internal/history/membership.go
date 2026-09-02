package history

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

var membershipSchemaReady sync.Map

const membershipSchema = `
CREATE TABLE IF NOT EXISTS trend_signal_memberships (
  trend_key TEXT NOT NULL,
  signal_id TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY (trend_key, signal_id),
  FOREIGN KEY (signal_id) REFERENCES signals(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_trend_signal_memberships_trend
  ON trend_signal_memberships(trend_key, observed_at DESC);
`

func (s *Store) ensureMembershipSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("history store is unavailable")
	}
	if _, ok := membershipSchemaReady.Load(s); ok {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, membershipSchema); err != nil {
		return fmt.Errorf("ensure trend membership schema: %w", err)
	}
	membershipSchemaReady.Store(s, struct{}{})
	return nil
}

// RecordTrendSignals preserves the evidence set that caused a signal cluster to
// resolve to a stable trend identity. The mapping is intentionally append-only
// at the signal level so later human labels can replay the exact observations
// that fed a detected trend instead of evaluating against reconstructed prose.
func (s *Store) RecordTrendSignals(ctx context.Context, trendKey string, values []model.Signal, observedAt time.Time) error {
	if trendKey == "" || len(values) == 0 {
		return nil
	}
	if err := s.ensureMembershipSchema(ctx); err != nil {
		return err
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, signal := range values {
		if signal.ID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO trend_signal_memberships (trend_key, signal_id, observed_at)
VALUES (?, ?, ?)
ON CONFLICT(trend_key, signal_id) DO UPDATE SET observed_at=excluded.observed_at`,
			trendKey, signal.ID, observedAt.UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record trend membership %s/%s: %w", trendKey, signal.ID, err)
		}
	}
	return tx.Commit()
}
