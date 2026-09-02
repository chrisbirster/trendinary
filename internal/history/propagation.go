package history

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

var propagationSchemaReady sync.Map

const propagationSchema = `
CREATE TABLE IF NOT EXISTS trend_source_observations (
  trend_id TEXT NOT NULL,
  source_domain TEXT NOT NULL,
  source_name TEXT NOT NULL,
  source_url TEXT,
  observed_at TEXT NOT NULL,
  first_seen TEXT NOT NULL,
  last_seen TEXT NOT NULL,
  signal_count INTEGER NOT NULL,
  engagement INTEGER NOT NULL,
  PRIMARY KEY (trend_id, source_domain, observed_at)
);
CREATE INDEX IF NOT EXISTS idx_trend_source_observations_trend
  ON trend_source_observations(trend_id, observed_at);
`

func (s *Store) ensurePropagationSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("history store is unavailable")
	}
	if _, ok := propagationSchemaReady.Load(s); ok {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, propagationSchema); err != nil {
		return fmt.Errorf("ensure propagation schema: %w", err)
	}
	propagationSchemaReady.Store(s, struct{}{})
	return nil
}

func (s *Store) RecordPropagation(ctx context.Context, trendID string, hops []model.PropagationHop, observedAt time.Time) error {
	if trendID == "" || len(hops) == 0 {
		return nil
	}
	if err := s.ensurePropagationSchema(ctx); err != nil {
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
	for _, hop := range hops {
		domain := hop.Source.Domain
		if domain == "" {
			domain = hop.Source.Name
		}
		if domain == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
INSERT OR REPLACE INTO trend_source_observations (
 trend_id, source_domain, source_name, source_url, observed_at,
 first_seen, last_seen, signal_count, engagement
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			trendID, domain, hop.Source.Name, hop.Source.URL,
			observedAt.UTC().Format(time.RFC3339Nano), hop.FirstSeen, hop.LastSeen,
			hop.SignalCount, hop.Engagement,
		); err != nil {
			return fmt.Errorf("record propagation %s/%s: %w", trendID, domain, err)
		}
	}
	return tx.Commit()
}

// Propagation returns the durable source path for a trend. First-seen is the
// earliest observation across scans; counts/engagement use the maximum observed
// cluster values so a shrinking rolling window cannot erase a prior breakout.
// V3 also annotates each hop with elapsed time from the first observed source.
func (s *Store) Propagation(ctx context.Context, trendID string) ([]model.PropagationHop, error) {
	if trendID == "" {
		return nil, nil
	}
	if err := s.ensurePropagationSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT source_domain, MAX(source_name), MAX(source_url), MIN(first_seen), MAX(last_seen),
       MAX(signal_count), MAX(engagement)
FROM trend_source_observations
WHERE trend_id = ?
GROUP BY source_domain`, trendID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.PropagationHop, 0)
	for rows.Next() {
		var hop model.PropagationHop
		if err := rows.Scan(
			&hop.Source.Domain, &hop.Source.Name, &hop.Source.URL,
			&hop.FirstSeen, &hop.LastSeen, &hop.SignalCount, &hop.Engagement,
		); err != nil {
			return nil, err
		}
		out = append(out, hop)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FirstSeen == out[j].FirstSeen {
			return out[i].Source.Domain < out[j].Source.Domain
		}
		return out[i].FirstSeen < out[j].FirstSeen
	})
	if len(out) == 0 {
		return out, nil
	}
	origin, err := time.Parse(time.RFC3339Nano, out[0].FirstSeen)
	if err != nil {
		if parsed, fallbackErr := time.Parse(time.RFC3339, out[0].FirstSeen); fallbackErr == nil {
			origin = parsed
		} else {
			return out, nil
		}
	}
	for index := range out {
		seen, err := time.Parse(time.RFC3339Nano, out[index].FirstSeen)
		if err != nil {
			seen, err = time.Parse(time.RFC3339, out[index].FirstSeen)
		}
		if err != nil {
			continue
		}
		minutes := int(seen.Sub(origin).Round(time.Minute) / time.Minute)
		if minutes < 0 {
			minutes = 0
		}
		out[index].DelayMinutes = minutes
		out[index].DelayLabel = propagationDelayLabel(minutes)
	}
	return out, nil
}

func propagationDelayLabel(minutes int) string {
	if minutes <= 0 {
		return "origin"
	}
	if minutes < 60 {
		return fmt.Sprintf("+%dm", minutes)
	}
	hours := minutes / 60
	remaining := minutes % 60
	if remaining == 0 {
		return fmt.Sprintf("+%dh", hours)
	}
	return fmt.Sprintf("+%dh %dm", hours, remaining)
}
