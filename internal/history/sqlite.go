package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type RawMetrics struct {
	SignalCount    int     `json:"signal_count"`
	SourceCount    int     `json:"source_count"`
	CommunityCount int     `json:"community_count"`
	RawAttention   float64 `json:"raw_attention"`
	RawEngagement  float64 `json:"raw_engagement"`
}

type Snapshot struct {
	TrendKey   string                `json:"trend_key"`
	ObservedAt time.Time             `json:"observed_at"`
	Lifecycle  string                `json:"lifecycle"`
	Score      engine.ScoreBreakdown `json:"score"`
	Raw        RawMetrics            `json:"raw"`
}

type Baseline struct {
	TrendKey        string    `json:"trend_key"`
	Observations    int       `json:"observations"`
	AverageAttention float64  `json:"average_attention"`
	MaximumAttention float64  `json:"maximum_attention"`
	FirstSeen       time.Time `json:"first_seen,omitempty"`
	LastSeen        time.Time `json:"last_seen,omitempty"`
	Latest          *Snapshot `json:"latest,omitempty"`
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single writer is plenty for the first Trendinary scanner and avoids
	// needless lock contention on a Fly volume. Reads still reuse the connection.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}
	if err := store.configure(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) configure(ctx context.Context) error {
	statements := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("sqlite configure %q: %w", statement, err)
		}
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	// This schema is still pre-production. Raw metrics are stored alongside the
	// normalized score inputs so future calibration can be replayed without
	// circularly deriving baselines from already-normalized values.
	const schema = `
CREATE TABLE IF NOT EXISTS signals (
  id TEXT PRIMARY KEY,
  source_name TEXT NOT NULL,
  source_domain TEXT,
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
CREATE INDEX IF NOT EXISTS idx_signals_observed_at ON signals(observed_at);
CREATE INDEX IF NOT EXISTS idx_signals_source_domain ON signals(source_domain);

CREATE TABLE IF NOT EXISTS trend_snapshots (
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
CREATE INDEX IF NOT EXISTS idx_trend_snapshots_observed_at ON trend_snapshots(observed_at);
CREATE INDEX IF NOT EXISTS idx_trend_snapshots_trend_time ON trend_snapshots(trend_key, observed_at DESC);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("sqlite migrate: %w", err)
	}
	return nil
}

func (s *Store) RecordSignals(ctx context.Context, values []model.Signal) error {
	if len(values) == 0 {
		return nil
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	const query = `
INSERT INTO signals (
  id, source_name, source_domain, title, body, url, author, published_at,
  observed_at, score, replies, likes, reposts, quotes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  source_name=excluded.source_name,
  source_domain=excluded.source_domain,
  title=excluded.title,
  body=excluded.body,
  url=excluded.url,
  author=excluded.author,
  published_at=excluded.published_at,
  observed_at=excluded.observed_at,
  score=excluded.score,
  replies=excluded.replies,
  likes=excluded.likes,
  reposts=excluded.reposts,
  quotes=excluded.quotes`

	observedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, signal := range values {
		if _, err := transaction.ExecContext(ctx, query,
			signal.ID,
			signal.Source.Name,
			signal.Source.Domain,
			signal.Title,
			signal.Text,
			signal.URL,
			signal.Author,
			signal.PublishedAt,
			observedAt,
			signal.Engagement.Score,
			signal.Engagement.Replies,
			signal.Engagement.Likes,
			signal.Engagement.Reposts,
			signal.Engagement.Quotes,
		); err != nil {
			return fmt.Errorf("record signal %s: %w", signal.ID, err)
		}
	}
	return transaction.Commit()
}

func (s *Store) RecordSnapshot(ctx context.Context, snapshot Snapshot) error {
	if snapshot.TrendKey == "" {
		return fmt.Errorf("trend key is required")
	}
	observedAt := snapshot.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO trend_snapshots (
  trend_key, observed_at, lifecycle, score, score_version, attention, velocity,
  source_breadth, community_breadth, novelty, confidence, signal_count,
  source_count, community_count, raw_attention, raw_engagement
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.TrendKey,
		observedAt.UTC().Format(time.RFC3339Nano),
		snapshot.Lifecycle,
		snapshot.Score.Score,
		snapshot.Score.Version,
		snapshot.Score.Attention,
		snapshot.Score.Velocity,
		snapshot.Score.SourceBreadth,
		snapshot.Score.CommunityBreadth,
		snapshot.Score.Novelty,
		snapshot.Score.Confidence,
		snapshot.Raw.SignalCount,
		snapshot.Raw.SourceCount,
		snapshot.Raw.CommunityCount,
		snapshot.Raw.RawAttention,
		snapshot.Raw.RawEngagement,
	)
	if err != nil {
		return fmt.Errorf("record snapshot: %w", err)
	}
	return nil
}

func (s *Store) LatestSnapshot(ctx context.Context, trendKey string) (Snapshot, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT observed_at, lifecycle, score, score_version, attention, velocity,
       source_breadth, community_breadth, novelty, confidence, signal_count,
       source_count, community_count, raw_attention, raw_engagement
FROM trend_snapshots
WHERE trend_key = ?
ORDER BY observed_at DESC
LIMIT 1`, trendKey)

	snapshot, err := scanSnapshot(row, trendKey)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	return snapshot, true, nil
}

func (s *Store) Baseline(ctx context.Context, trendKey string, since time.Time) (Baseline, error) {
	if since.IsZero() {
		since = time.Now().UTC().Add(-7 * 24 * time.Hour)
	}
	var baseline Baseline
	baseline.TrendKey = trendKey
	var firstSeen, lastSeen sql.NullString
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(AVG(raw_attention), 0), COALESCE(MAX(raw_attention), 0),
       MIN(observed_at), MAX(observed_at)
FROM trend_snapshots
WHERE trend_key = ? AND observed_at >= ?`, trendKey, since.UTC().Format(time.RFC3339Nano)).Scan(
		&baseline.Observations,
		&baseline.AverageAttention,
		&baseline.MaximumAttention,
		&firstSeen,
		&lastSeen,
	); err != nil {
		return Baseline{}, fmt.Errorf("baseline: %w", err)
	}
	if firstSeen.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, firstSeen.String)
		if err != nil {
			return Baseline{}, err
		}
		baseline.FirstSeen = parsed
	}
	if lastSeen.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, lastSeen.String)
		if err != nil {
			return Baseline{}, err
		}
		baseline.LastSeen = parsed
	}
	if latest, ok, err := s.LatestSnapshot(ctx, trendKey); err != nil {
		return Baseline{}, err
	} else if ok {
		baseline.Latest = &latest
	}
	return baseline, nil
}

func (s *Store) RecentSnapshots(ctx context.Context, trendKey string, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 24
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT observed_at, lifecycle, score, score_version, attention, velocity,
       source_breadth, community_breadth, novelty, confidence, signal_count,
       source_count, community_count, raw_attention, raw_engagement
FROM trend_snapshots
WHERE trend_key = ?
ORDER BY observed_at DESC
LIMIT ?`, trendKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Snapshot, 0, limit)
	for rows.Next() {
		snapshot, err := scanSnapshot(rows, trendKey)
		if err != nil {
			return nil, err
		}
		out = append(out, snapshot)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(row rowScanner, trendKey string) (Snapshot, error) {
	var observed string
	var snapshot Snapshot
	snapshot.TrendKey = trendKey
	if err := row.Scan(
		&observed,
		&snapshot.Lifecycle,
		&snapshot.Score.Score,
		&snapshot.Score.Version,
		&snapshot.Score.Attention,
		&snapshot.Score.Velocity,
		&snapshot.Score.SourceBreadth,
		&snapshot.Score.CommunityBreadth,
		&snapshot.Score.Novelty,
		&snapshot.Score.Confidence,
		&snapshot.Raw.SignalCount,
		&snapshot.Raw.SourceCount,
		&snapshot.Raw.CommunityCount,
		&snapshot.Raw.RawAttention,
		&snapshot.Raw.RawEngagement,
	); err != nil {
		return Snapshot{}, err
	}
	timestamp, err := time.Parse(time.RFC3339Nano, observed)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.ObservedAt = timestamp
	return snapshot, nil
}
