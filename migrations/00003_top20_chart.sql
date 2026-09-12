-- +goose Up
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

-- +goose Down
DROP INDEX IF EXISTS idx_trend_chart_entries_scan_rank;
DROP INDEX IF EXISTS idx_trend_chart_entries_trend_time;
DROP TABLE IF EXISTS trend_chart_entries;
