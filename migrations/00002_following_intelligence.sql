-- +goose Up

CREATE TABLE IF NOT EXISTS following_alert_context (
  alert_id TEXT PRIMARY KEY,
  radar_id TEXT NOT NULL,
  lifecycle_before TEXT NOT NULL,
  lifecycle_after TEXT NOT NULL,
  score_before INTEGER NOT NULL,
  score_after INTEGER NOT NULL,
  velocity_before REAL NOT NULL,
  velocity_after REAL NOT NULL,
  source_count_before INTEGER NOT NULL,
  source_count_after INTEGER NOT NULL,
  source_breadth_before REAL NOT NULL,
  source_breadth_after REAL NOT NULL,
  observed_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_following_alert_context_radar ON following_alert_context(radar_id);

CREATE TABLE IF NOT EXISTS following_alert_feedback (
  alert_id TEXT PRIMARY KEY,
  radar_id TEXT NOT NULL,
  rating TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_following_alert_feedback_radar ON following_alert_feedback(radar_id);

CREATE TABLE IF NOT EXISTS following_alert_delivery (
  alert_id TEXT PRIMARY KEY,
  radar_id TEXT NOT NULL,
  push_attempts INTEGER NOT NULL,
  status TEXT NOT NULL,
  last_error TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_following_alert_delivery_radar ON following_alert_delivery(radar_id);

CREATE TABLE IF NOT EXISTS following_briefing_checkpoint (
  radar_id TEXT PRIMARY KEY,
  seen_at TEXT NOT NULL
);
