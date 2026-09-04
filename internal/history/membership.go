package history

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

var membershipSchemaReady sync.Map

const membershipSchema = `
CREATE TABLE IF NOT EXISTS trend_signal_memberships (
  trend_key TEXT NOT NULL,
  signal_id TEXT NOT NULL,
  first_observed_at TEXT NOT NULL DEFAULT '',
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
	if err := s.ensureMembershipFirstObservedAt(ctx); err != nil {
		return err
	}
	membershipSchemaReady.Store(s, struct{}{})
	return nil
}

func (s *Store) ensureMembershipFirstObservedAt(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(trend_signal_memberships)`)
	if err != nil {
		return fmt.Errorf("inspect trend membership schema: %w", err)
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return fmt.Errorf("scan trend membership schema: %w", err)
		}
		if name == "first_observed_at" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("inspect trend membership schema rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !found {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE trend_signal_memberships ADD COLUMN first_observed_at TEXT NOT NULL DEFAULT ''`); err != nil {
			// During a rolling Fly deployment another process may have completed
			// the same additive migration after this process inspected the table.
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				return fmt.Errorf("add trend membership first observation: %w", err)
			}
		}
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE trend_signal_memberships
SET first_observed_at = observed_at
WHERE first_observed_at = ''`); err != nil {
		return fmt.Errorf("backfill trend membership first observation: %w", err)
	}
	return nil
}

// RecordTrendSignals preserves both the first time evidence resolved into a
// stable trend and the most recent time it remained relevant. The latter is
// refreshed for rolling windows; the former is immutable discovery timing used
// by source-contribution analytics.
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
	stamp := observedAt.UTC().Format(time.RFC3339Nano)
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
INSERT INTO trend_signal_memberships (trend_key, signal_id, first_observed_at, observed_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(trend_key, signal_id) DO UPDATE SET observed_at=excluded.observed_at`,
			trendKey, signal.ID, stamp, stamp); err != nil {
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
       COALESCE(sig.discovery_channel, ''),
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
			&signal.DiscoveryChannel,
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
