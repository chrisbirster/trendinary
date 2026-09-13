package history

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
// persists that scan atomically. Remote libSQL makes one-query-per-trend chart
// history prohibitively expensive, so the history reads and scan write are
// grouped into a small fixed number of round trips regardless of chart size.
func (s *Store) ApplyChart(ctx context.Context, trends []model.Trend, scanAt time.Time) ([]model.Trend, error) {
	if len(trends) == 0 {
		return trends, nil
	}
	if err := s.ensureChartSchema(ctx); err != nil {
		return nil, err
	}
	scanAt = scanAt.UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	previousScan, err := latestChartScanBeforeTx(ctx, tx, scanAt)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(trends))
	for _, trend := range trends {
		ids = append(ids, chartTrendID(trend))
	}

	aggregates, err := chartAggregatesTx(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	previousRanks := map[string]int{}
	consecutive := map[string]int{}
	if previousScan != nil {
		previousRanks, err = chartPreviousRanksTx(ctx, tx, ids, *previousScan)
		if err != nil {
			return nil, err
		}
		consecutive, err = chartConsecutiveScansTx(ctx, tx, ids, *previousScan)
		if err != nil {
			return nil, err
		}
	}

	out := append([]model.Trend(nil), trends...)
	for index := range out {
		trend := &out[index]
		trendID := chartTrendID(*trend)
		stats := aggregates[trendID]
		stats.PreviousRank = previousRanks[trendID]
		stats.ConsecutiveScans = consecutive[trendID]

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
	}

	if err := recordChartEntriesTx(ctx, tx, out, scanAt); err != nil {
		return nil, err
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

func chartTrendID(trend model.Trend) string {
	if trend.ID != "" {
		return trend.ID
	}
	return trend.Slug
}

func chartPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func stringArgs(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func latestChartScanBeforeTx(ctx context.Context, tx *sql.Tx, before time.Time) (*time.Time, error) {
	var raw sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT MAX(scan_at) FROM trend_chart_entries WHERE scan_at < ?`, before.UTC().Format(time.RFC3339Nano)).Scan(&raw); err != nil {
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

func chartAggregatesTx(ctx context.Context, tx *sql.Tx, trendIDs []string) (map[string]chartAggregate, error) {
	out := make(map[string]chartAggregate, len(trendIDs))
	if len(trendIDs) == 0 {
		return out, nil
	}
	query := `
SELECT trend_id, COALESCE(MIN(rank), 0), COUNT(*),
       COALESCE(SUM(CASE WHEN rank = 1 THEN 1 ELSE 0 END), 0)
FROM trend_chart_entries
WHERE trend_id IN (` + chartPlaceholders(len(trendIDs)) + `)
GROUP BY trend_id`
	rows, err := tx.QueryContext(ctx, query, stringArgs(trendIDs)...)
	if err != nil {
		return nil, fmt.Errorf("chart aggregates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var trendID string
		var stats chartAggregate
		if err := rows.Scan(&trendID, &stats.PeakRank, &stats.TotalScans, &stats.NumberOneScans); err != nil {
			return nil, fmt.Errorf("scan chart aggregate: %w", err)
		}
		out[trendID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chart aggregates: %w", err)
	}
	return out, nil
}

func chartPreviousRanksTx(ctx context.Context, tx *sql.Tx, trendIDs []string, previousScan time.Time) (map[string]int, error) {
	out := make(map[string]int, len(trendIDs))
	if len(trendIDs) == 0 {
		return out, nil
	}
	args := []any{previousScan.UTC().Format(time.RFC3339Nano)}
	args = append(args, stringArgs(trendIDs)...)
	query := `SELECT trend_id, rank FROM trend_chart_entries WHERE scan_at = ? AND trend_id IN (` + chartPlaceholders(len(trendIDs)) + `)`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chart previous ranks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var trendID string
		var rank int
		if err := rows.Scan(&trendID, &rank); err != nil {
			return nil, fmt.Errorf("scan chart previous rank: %w", err)
		}
		out[trendID] = rank
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chart previous ranks: %w", err)
	}
	return out, nil
}

func chartConsecutiveScansTx(ctx context.Context, tx *sql.Tx, trendIDs []string, previousScan time.Time) (map[string]int, error) {
	out := make(map[string]int, len(trendIDs))
	if len(trendIDs) == 0 {
		return out, nil
	}
	args := []any{previousScan.UTC().Format(time.RFC3339Nano)}
	args = append(args, stringArgs(trendIDs)...)
	query := `
WITH recent_scan_values AS (
  SELECT DISTINCT scan_at
  FROM trend_chart_entries
  WHERE scan_at <= ?
  ORDER BY scan_at DESC
  LIMIT 500
),
recent_scans AS (
  SELECT scan_at, ROW_NUMBER() OVER (ORDER BY scan_at DESC) AS seq
  FROM recent_scan_values
),
occurrences AS (
  SELECT e.trend_id,
         rs.seq,
         ROW_NUMBER() OVER (PARTITION BY e.trend_id ORDER BY rs.seq) AS occurrence
  FROM recent_scans rs
  JOIN trend_chart_entries e ON e.scan_at = rs.scan_at
  WHERE e.trend_id IN (` + chartPlaceholders(len(trendIDs)) + `)
)
SELECT trend_id,
       COALESCE(MIN(CASE WHEN seq <> occurrence THEN occurrence - 1 END), COUNT(*)) AS consecutive_scans
FROM occurrences
GROUP BY trend_id`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chart consecutive scans: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var trendID string
		var count int
		if err := rows.Scan(&trendID, &count); err != nil {
			return nil, fmt.Errorf("scan chart consecutive count: %w", err)
		}
		out[trendID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chart consecutive scans: %w", err)
	}
	return out, nil
}

func recordChartEntriesTx(ctx context.Context, tx *sql.Tx, trends []model.Trend, scanAt time.Time) error {
	if len(trends) == 0 {
		return nil
	}
	var query strings.Builder
	query.WriteString(`
INSERT INTO trend_chart_entries (
  scan_at, trend_id, slug, rank, score, confidence_tier,
  publisher_count, platform_count, signal_count
) VALUES `)
	args := make([]any, 0, len(trends)*9)
	for i, trend := range trends {
		if i > 0 {
			query.WriteString(",")
		}
		query.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			scanAt.UTC().Format(time.RFC3339Nano), chartTrendID(trend), trend.Slug, trend.Rank, trend.Score,
			trend.ConfidenceTier, trend.Provenance.PublisherCount, trend.Provenance.PlatformCount,
			trend.Provenance.SignalCount,
		)
	}
	query.WriteString(`
ON CONFLICT(scan_at, trend_id) DO UPDATE SET
  slug=excluded.slug,
  rank=excluded.rank,
  score=excluded.score,
  confidence_tier=excluded.confidence_tier,
  publisher_count=excluded.publisher_count,
  platform_count=excluded.platform_count,
  signal_count=excluded.signal_count`)
	if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
		return fmt.Errorf("record chart entries: %w", err)
	}
	return nil
}
