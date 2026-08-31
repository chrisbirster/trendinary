package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Snapshot struct {
	TrendKey   string                `json:"trend_key"`
	ObservedAt time.Time             `json:"observed_at"`
	Lifecycle  string                `json:"lifecycle"`
	Score      engine.ScoreBreakdown `json:"score"`
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
  PRIMARY KEY (trend_key, observed_at)
);
CREATE INDEX IF NOT EXISTS idx_trend_snapshots_observed_at ON trend_snapshots(observed_at);
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
  source_breadth, community_breadth, novelty, confidence
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
	)
	if err != nil {
		return fmt.Errorf("record snapshot: %w", err)
	}
	return nil
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
       source_breadth, community_breadth, novelty, confidence
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
		var observed string
		var snapshot Snapshot
		snapshot.TrendKey = trendKey
		if err := rows.Scan(
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
		); err != nil {
			return nil, err
		}
		timestamp, err := time.Parse(time.RFC3339Nano, observed)
		if err != nil {
			return nil, err
		}
		snapshot.ObservedAt = timestamp
		out = append(out, snapshot)
	}
	return out, rows.Err()
}
