package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

const localChartSchema = `
CREATE TABLE IF NOT EXISTS trend_chart_entries (
  scan_at TEXT NOT NULL,
  trend_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  rank INTEGER NOT NULL,
  score INTEGER NOT NULL,
  confidence_tier TEXT NOT NULL DEFAULT 'EMERGING',
  publisher_count INTEGER NOT NULL DEFAULT 0,
  platform_count INTEGER NOT NULL DEFAULT 0,
  signal_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (scan_at, trend_id)
);
CREATE INDEX IF NOT EXISTS idx_trend_chart_entries_trend_time
  ON trend_chart_entries(trend_id, scan_at DESC);
CREATE INDEX IF NOT EXISTS idx_trend_chart_entries_scan_rank
  ON trend_chart_entries(scan_at DESC, rank);
`

func (s *Store) ensureChartSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is required")
	}
	if s.ExternallyManagedSchema() {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, localChartSchema); err != nil {
		return fmt.Errorf("create local chart schema: %w", err)
	}
	return nil
}

// ApplyChart annotates a newly ranked chart with movement/history metadata and
// persists that scan atomically. The chart table intentionally stores only
// ranking facts; full trend/evidence state remains in the existing stores.
func (s *Store) ApplyChart(ctx context.Context, trends []model.Trend, scanAt time.Time) ([]model.Trend, error) {
	if len(trends) == 0 {
		return trends, nil
	}
	if err := s.ensureChartSchema(ctx); err != nil {
		return nil, err
	}
	scanAt = scanAt.UTC()
	previousScan, err := s.latestChartScanBefore(ctx, scanAt)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	out := append([]model.Trend(nil), trends...)
	for index := range out {
		trend := &out[index]
		trendID := trend.ID
		if trendID == "" {
			trendID = trend.Slug
		}
		stats, err := chartStatsTx(ctx, tx, trendID, previousScan)
		if err != nil {
			return nil, err
		}

		chart := model.ChartStats{}
		chart.PreviousRank = stats.PreviousRank
		chart.PeakRank = trend.Rank
		if stats.PeakRank > 0 && stats.PeakRank < chart.PeakRank {
			chart.PeakRank = stats.PeakRank
		}
		chart.TotalScans = stats.TotalScans + 1
		chart.NumberOneScans = stats.NumberOneScans
		if trend.Rank == 1 {
			chart.NumberOneScans++
		}

		switch {
		case stats.TotalScans == 0:
			chart.Status = "NEW"
			chart.Movement = "NEW"
			chart.ConsecutiveScans = 1
		case stats.PreviousRank == 0:
			chart.Status = "RE"
			chart.Movement = "RE"
			chart.ConsecutiveScans = 1
		default:
			chart.ConsecutiveScans = stats.ConsecutiveScans + 1
			delta := stats.PreviousRank - trend.Rank
			switch {
			case delta > 0:
				chart.Movement = fmt.Sprintf("▲ %d", delta)
			case delta < 0:
				chart.Movement = fmt.Sprintf("▼ %d", -delta)
			default:
				chart.Movement = "—"
			}
		}
		trend.Chart = chart

		_, err = tx.ExecContext(ctx, `
INSERT INTO trend_chart_entries (
  scan_at, trend_id, slug, rank, score, confidence_tier,
  publisher_count, platform_count, signal_count
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(scan_at, trend_id) DO UPDATE SET
  slug=excluded.slug,
  rank=excluded.rank,
  score=excluded.score,
  confidence_tier=excluded.confidence_tier,
  publisher_count=excluded.publisher_count,
  platform_count=excluded.platform_count,
  signal_count=excluded.signal_count`,
			scanAt.Format(time.RFC3339Nano), trendID, trend.Slug, trend.Rank, trend.Score,
			trend.ConfidenceTier, trend.Provenance.PublisherCount, trend.Provenance.PlatformCount,
			trend.Provenance.SignalCount,
		)
		if err != nil {
			return nil, fmt.Errorf("record chart entry %s: %w", trendID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

type chartAggregate struct {
	PreviousRank     int
	PeakRank         int
	TotalScans       int
	ConsecutiveScans int
	NumberOneScans   int
}

func chartStatsTx(ctx context.Context, tx *sql.Tx, trendID string, previousScan *time.Time) (chartAggregate, error) {
	var stats chartAggregate
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(MIN(rank), 0), COUNT(*), COALESCE(SUM(CASE WHEN rank = 1 THEN 1 ELSE 0 END), 0)
FROM trend_chart_entries
WHERE trend_id = ?`, trendID).Scan(&stats.PeakRank, &stats.TotalScans, &stats.NumberOneScans); err != nil {
		return stats, fmt.Errorf("chart aggregate %s: %w", trendID, err)
	}
	if previousScan == nil {
		return stats, nil
	}
	_ = tx.QueryRowContext(ctx, `SELECT rank FROM trend_chart_entries WHERE trend_id = ? AND scan_at = ?`,
		trendID, previousScan.Format(time.RFC3339Nano)).Scan(&stats.PreviousRank)
	if stats.PreviousRank == 0 {
		return stats, nil
	}

	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT scan_at FROM trend_chart_entries WHERE scan_at <= ? ORDER BY scan_at DESC LIMIT 500`, previousScan.Format(time.RFC3339Nano))
	if err != nil {
		return stats, fmt.Errorf("chart scans: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var scan string
		if err := rows.Scan(&scan); err != nil {
			return stats, err
		}
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM trend_chart_entries WHERE trend_id = ? AND scan_at = ?`, trendID, scan).Scan(&exists); err != nil {
			return stats, err
		}
		if exists == 0 {
			break
		}
		stats.ConsecutiveScans++
	}
	return stats, rows.Err()
}

func (s *Store) latestChartScanBefore(ctx context.Context, before time.Time) (*time.Time, error) {
	var raw sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(scan_at) FROM trend_chart_entries WHERE scan_at < ?`, before.UTC().Format(time.RFC3339Nano)).Scan(&raw); err != nil {
		return nil, fmt.Errorf("latest chart scan: %w", err)
	}
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		return nil, fmt.Errorf("parse latest chart scan: %w", err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
