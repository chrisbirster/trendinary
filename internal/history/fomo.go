package history

import (
	"context"
	"fmt"
	"time"
)

type FomoItem struct {
	TrendKey      string  `json:"trend_key"`
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	PeakScore     int     `json:"peak_score"`
	FirstSeen     string  `json:"first_seen"`
	LastSeen      string  `json:"last_seen"`
	MaxVelocity   float64 `json:"max_velocity"`
	SourceBreadth float64 `json:"source_breadth"`
	Novelty       float64 `json:"novelty"`
	Summary       string  `json:"summary"`
}

// FomoBriefing returns a finite, history-backed catch-up list rather than an
// infinite feed. Peak score chooses importance while velocity/breadth explain
// why an item made the cut.
func (s *Store) FomoBriefing(ctx context.Context, since time.Time, limit int) ([]FomoItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("history store is unavailable")
	}
	if err := s.ensureEntitySchema(ctx); err != nil {
		return nil, err
	}
	if since.IsZero() {
		since = time.Now().UTC().Add(-24 * time.Hour)
	}
	if limit <= 0 {
		limit = 7
	}
	if limit > 25 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT ts.trend_key,
       COALESCE(te.slug, ts.trend_key),
       COALESCE(te.canonical_name, ts.trend_key),
       MAX(ts.score), MIN(ts.observed_at), MAX(ts.observed_at),
       MAX(ts.velocity), MAX(ts.source_breadth), MAX(ts.novelty)
FROM trend_snapshots ts
LEFT JOIN trend_entities te ON te.id = ts.trend_key
WHERE ts.observed_at >= ?
GROUP BY ts.trend_key, te.slug, te.canonical_name
ORDER BY MAX(ts.score) DESC, MAX(ts.velocity) DESC
LIMIT ?`, since.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, fmt.Errorf("fomo briefing: %w", err)
	}
	defer rows.Close()
	out := make([]FomoItem, 0, limit)
	for rows.Next() {
		var item FomoItem
		if err := rows.Scan(
			&item.TrendKey, &item.Slug, &item.Name, &item.PeakScore,
			&item.FirstSeen, &item.LastSeen, &item.MaxVelocity, &item.SourceBreadth, &item.Novelty,
		); err != nil {
			return nil, err
		}
		item.Summary = fomoSummary(item)
		out = append(out, item)
	}
	return out, rows.Err()
}

func fomoSummary(item FomoItem) string {
	switch {
	case item.MaxVelocity >= 0.75 && item.SourceBreadth >= 0.40:
		return "Fast acceleration crossed multiple independent sources."
	case item.MaxVelocity >= 0.75:
		return "Attention accelerated unusually quickly relative to its baseline."
	case item.SourceBreadth >= 0.40:
		return "The topic spread across an unusually broad set of sources."
	case item.Novelty >= 0.70:
		return "A relatively novel topic broke above its historical baseline."
	default:
		return "It reached one of the strongest Trendinary scores in this catch-up window."
	}
}
