PRAGMA foreign_keys = ON;

CREATE TABLE signals (
  id TEXT PRIMARY KEY,
  source_name TEXT NOT NULL,
  source_domain TEXT,
  discovery_channel TEXT NOT NULL DEFAULT '',
  title TEXT,
  body TEXT,
  url TEXT,
  author TEXT,
  published_at TEXT,
  observed_at TEXT NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  replies INTEGER NOT NULL DEFAULT 0,
  likes INTEGER NOT NULL DEFAULT 0,
  reposts INTEGER NOT NULL DEFAULT 0,
  quotes INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_signals_observed_at ON signals(observed_at);
CREATE INDEX idx_signals_source_domain ON signals(source_domain);

CREATE TABLE trend_snapshots (
  trend_key TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  lifecycle TEXT NOT NULL,
  score INTEGER NOT NULL,
  score_version TEXT NOT NULL,
  attention REAL NOT NULL,
  velocity REAL NOT NULL,
  source_breadth REAL NOT NULL,
  community_breadth REAL NOT NULL,
  novelty REAL NOT NULL,
  confidence REAL NOT NULL,
  signal_count INTEGER NOT NULL DEFAULT 0,
  source_count INTEGER NOT NULL DEFAULT 0,
  community_count INTEGER NOT NULL DEFAULT 0,
  raw_attention REAL NOT NULL DEFAULT 0,
  raw_engagement REAL NOT NULL DEFAULT 0,
  PRIMARY KEY (trend_key, observed_at)
);
CREATE INDEX idx_trend_snapshots_observed_at ON trend_snapshots(observed_at);
CREATE INDEX idx_trend_snapshots_trend_time ON trend_snapshots(trend_key, observed_at DESC);

CREATE TABLE stream_cursors (
  name TEXT PRIMARY KEY,
  seq INTEGER NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE trend_entities (
  id TEXT PRIMARY KEY,
  slug TEXT NOT NULL,
  canonical_name TEXT NOT NULL,
  first_seen TEXT NOT NULL,
  last_seen TEXT NOT NULL
);
CREATE UNIQUE INDEX trend_entities_slug ON trend_entities(slug);
CREATE TABLE trend_entity_terms (
  entity_id TEXT NOT NULL,
  term TEXT NOT NULL,
  PRIMARY KEY (entity_id, term),
  FOREIGN KEY (entity_id) REFERENCES trend_entities(id) ON DELETE CASCADE
);
CREATE INDEX idx_trend_entity_terms_term ON trend_entity_terms(term);
CREATE TABLE trend_entity_aliases (
  entity_id TEXT NOT NULL,
  alias_key TEXT NOT NULL,
  display_alias TEXT NOT NULL,
  PRIMARY KEY (entity_id, alias_key),
  FOREIGN KEY (entity_id) REFERENCES trend_entities(id) ON DELETE CASCADE
);
CREATE INDEX idx_trend_entity_aliases_key ON trend_entity_aliases(alias_key);

CREATE TABLE trend_signal_memberships (
  trend_key TEXT NOT NULL,
  signal_id TEXT NOT NULL,
  first_observed_at TEXT NOT NULL DEFAULT '',
  observed_at TEXT NOT NULL,
  PRIMARY KEY (trend_key, signal_id),
  FOREIGN KEY (signal_id) REFERENCES signals(id) ON DELETE CASCADE
);
CREATE INDEX idx_trend_signal_memberships_trend ON trend_signal_memberships(trend_key, observed_at DESC);

CREATE TABLE trend_source_observations (
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
CREATE INDEX idx_trend_source_observations_trend ON trend_source_observations(trend_id, observed_at);

CREATE TABLE source_api_daily_quota (
  source TEXT NOT NULL,
  day TEXT NOT NULL,
  calls INTEGER NOT NULL DEFAULT 0,
  last_call_at TEXT,
  next_call_at TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (source, day)
);
CREATE INDEX idx_source_api_daily_quota_day ON source_api_daily_quota(day);

CREATE TABLE editorial_sources (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  url TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  last_attempted_at TEXT,
  last_successful_at TEXT,
  latest_item_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE content_items (
  id TEXT PRIMARY KEY,
  canonical_url TEXT NOT NULL,
  original_url TEXT NOT NULL,
  title TEXT NOT NULL,
  publisher TEXT NOT NULL DEFAULT '',
  publisher_domain TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT 'article',
  description TEXT NOT NULL DEFAULT '',
  image_url TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  published_at TEXT,
  discovered_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  editorial_state TEXT NOT NULL DEFAULT 'inbox',
  opened_at TEXT,
  queued_at TEXT,
  consumed_at TEXT,
  saved_at TEXT,
  rejected_at TEXT,
  editorial_score INTEGER NOT NULL DEFAULT 0,
  freshness REAL NOT NULL DEFAULT 0.5,
  personal_relevance REAL NOT NULL DEFAULT 0.5,
  novelty REAL NOT NULL DEFAULT 0.5,
  source_quality REAL NOT NULL DEFAULT 0.5,
  trend_signal REAL NOT NULL DEFAULT 0.5,
  serendipity_score REAL NOT NULL DEFAULT 0,
  serendipity INTEGER NOT NULL DEFAULT 0,
  why_interesting TEXT NOT NULL DEFAULT '',
  enrichment_status TEXT NOT NULL DEFAULT 'pending',
  enrichment_error TEXT NOT NULL DEFAULT '',
  estimated_minutes INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX content_items_canonical_url ON content_items(canonical_url);
CREATE INDEX idx_content_state_score ON content_items(editorial_state, editorial_score DESC, discovered_at DESC);
CREATE INDEX idx_content_discovered ON content_items(discovered_at DESC);
CREATE TABLE content_discoveries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  content_item_id TEXT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
  discovery_source_id TEXT NOT NULL REFERENCES editorial_sources(id),
  external_source_name TEXT NOT NULL DEFAULT '',
  source_age_text TEXT NOT NULL DEFAULT '',
  discovered_at TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}'
);
CREATE UNIQUE INDEX content_discoveries_content_item_id_discovery_source_id_external_source_name ON content_discoveries(content_item_id, discovery_source_id, external_source_name);
CREATE INDEX idx_discoveries_item ON content_discoveries(content_item_id, discovered_at DESC);
CREATE TABLE content_notes (
  content_item_id TEXT PRIMARY KEY REFERENCES content_items(id) ON DELETE CASCADE,
  note TEXT NOT NULL DEFAULT '',
  worth_sharing INTEGER,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE ingestion_runs (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL REFERENCES editorial_sources(id),
  started_at TEXT NOT NULL,
  completed_at TEXT,
  status TEXT NOT NULL,
  sections_seen INTEGER NOT NULL DEFAULT 0,
  items_seen INTEGER NOT NULL DEFAULT 0,
  items_inserted INTEGER NOT NULL DEFAULT 0,
  items_updated INTEGER NOT NULL DEFAULT 0,
  duplicates INTEGER NOT NULL DEFAULT 0,
  malformed INTEGER NOT NULL DEFAULT 0,
  errors INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_ingestion_source_time ON ingestion_runs(source_id, started_at DESC);
CREATE TABLE newsletter_issues (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  issue_date TEXT NOT NULL DEFAULT '',
  intro TEXT NOT NULL DEFAULT '',
  question TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE newsletter_issue_items (
  id TEXT PRIMARY KEY,
  issue_id TEXT NOT NULL REFERENCES newsletter_issues(id) ON DELETE CASCADE,
  content_item_id TEXT NOT NULL REFERENCES content_items(id),
  section TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  editor_note TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX newsletter_issue_items_issue_id_content_item_id_section ON newsletter_issue_items(issue_id, content_item_id, section);
CREATE INDEX idx_issue_items_order ON newsletter_issue_items(issue_id, section, position);

CREATE TABLE quality_feedback (
  id TEXT PRIMARY KEY,
  trend_key TEXT NOT NULL,
  trend_slug TEXT NOT NULL,
  trend_name TEXT NOT NULL,
  label TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  score INTEGER NOT NULL DEFAULT 0,
  lifecycle TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX idx_quality_feedback_trend_time ON quality_feedback(trend_key, created_at DESC);
CREATE INDEX idx_quality_feedback_label_time ON quality_feedback(label, created_at DESC);

CREATE TABLE following_radars (
  radar_id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  sensitivity TEXT NOT NULL DEFAULT 'balanced',
  lifecycle INTEGER NOT NULL DEFAULT 1,
  velocity INTEGER NOT NULL DEFAULT 1,
  corroboration INTEGER NOT NULL DEFAULT 1,
  resurfacing INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE following_follows (
  id TEXT PRIMARY KEY,
  radar_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  value TEXT NOT NULL,
  display_name TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX following_follows_radar_id_kind_value ON following_follows(radar_id, kind, value);
CREATE INDEX idx_following_follows_radar ON following_follows(radar_id);
CREATE TABLE following_baselines (
  follow_id TEXT NOT NULL,
  trend_key TEXT NOT NULL,
  slug TEXT NOT NULL,
  lifecycle TEXT NOT NULL,
  score INTEGER NOT NULL,
  velocity REAL NOT NULL,
  source_breadth REAL NOT NULL,
  source_count INTEGER NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY(follow_id, trend_key)
);
CREATE TABLE following_alerts (
  id TEXT PRIMARY KEY,
  radar_id TEXT NOT NULL,
  follow_id TEXT NOT NULL,
  trend_key TEXT NOT NULL,
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TEXT NOT NULL,
  read_at TEXT
);
CREATE UNIQUE INDEX following_alerts_radar_id_fingerprint ON following_alerts(radar_id, fingerprint);
CREATE INDEX idx_following_alerts_radar_created ON following_alerts(radar_id, created_at DESC);
CREATE TABLE following_push_subscriptions (
  id TEXT PRIMARY KEY,
  radar_id TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  p256dh TEXT NOT NULL,
  auth TEXT NOT NULL,
  created_at TEXT NOT NULL,
  last_success_at TEXT,
  failures INTEGER NOT NULL DEFAULT 0,
  disabled_at TEXT
);
CREATE UNIQUE INDEX following_push_subscriptions_radar_id_endpoint ON following_push_subscriptions(radar_id, endpoint);
CREATE INDEX idx_following_push_radar ON following_push_subscriptions(radar_id);
CREATE TABLE following_kv (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
