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
// resolve to a stable trend identity. Membership observation time is refreshed
// when the same evidence is seen again so rolling windows stay bounded.
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

// TrendSignals returns durable evidence recently associated with a stable trend.
// The caller still re-checks event similarity against the current cluster so a
// past bad membership cannot poison a corrected trend forever.
func (s *Store) TrendSignals(ctx context.Context, trendKey string, since time.Time, limit int) ([]model.Signal, error) {
	if trendKey == "" {
		return nil, nil
	}
	if err := s.ensureMembershipSchema(ctx); err != nil {
		return nil, err
	}
	if since.IsZero() {
		since = time.Now().UTC().Add(-24 * time.Hour)
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT sig.id,
       sig.source_name,
       COALESCE(sig.source_domain, ''),
       COALESCE(sig.title, ''),
       COALESCE(sig.body, ''),
       COALESCE(sig.url, ''),
       COALESCE(sig.author, ''),
       COALESCE(sig.published_at, ''),
       sig.score,
       sig.replies,
       sig.likes,
       sig.reposts,
       sig.quotes
FROM trend_signal_memberships m
JOIN signals sig ON sig.id = m.signal_id
WHERE m.trend_key = ? AND m.observed_at >= ?
ORDER BY m.observed_at DESC
LIMIT ?`, trendKey, since.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, fmt.Errorf("trend signals: %w", err)
	}
	defer rows.Close()
	out := make([]model.Signal, 0)
	for rows.Next() {
		var signal model.Signal
		if err := rows.Scan(
			&signal.ID,
			&signal.Source.Name,
			&signal.Source.Domain,
			&signal.Title,
			&signal.Text,
			&signal.URL,
			&signal.Author,
			&signal.PublishedAt,
			&signal.Engagement.Score,
			&signal.Engagement.Replies,
			&signal.Engagement.Likes,
			&signal.Engagement.Reposts,
			&signal.Engagement.Quotes,
		); err != nil {
			return nil, fmt.Errorf("scan trend signal: %w", err)
		}
		signal.Source.URL = signal.URL
		out = append(out, signal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("trend signals rows: %w", err)
	}
	return out, nil
}
