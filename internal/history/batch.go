package history

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

const (
	signalWriteBatchSize     = 32
	membershipWriteBatchSize = 100
)

// RecordSignalsBatch is the remote-libSQL-friendly scanner/stream write path.
// It preserves RecordSignals semantics while collapsing dozens of network
// round trips into bounded multi-row UPSERT statements. The conservative batch
// size stays below SQLite's common bind-variable limits.
func (s *Store) RecordSignalsBatch(ctx context.Context, values []model.Signal) error {
	if len(values) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	observedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for start := 0; start < len(values); start += signalWriteBatchSize {
		end := start + signalWriteBatchSize
		if end > len(values) {
			end = len(values)
		}
		chunk := values[start:end]

		var query strings.Builder
		query.WriteString(`INSERT INTO signals (
  id, source_name, source_domain, discovery_channel, title, body, url, author, published_at,
  observed_at, score, replies, likes, reposts, quotes
) VALUES `)
		args := make([]any, 0, len(chunk)*15)
		for i, signal := range chunk {
			if i > 0 {
				query.WriteByte(',')
			}
			query.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			args = append(args,
				signal.ID,
				signal.Source.Name,
				signal.Source.Domain,
				signal.DiscoveryChannel,
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
			)
		}
		query.WriteString(`
ON CONFLICT(id) DO UPDATE SET
  source_name=excluded.source_name,
  source_domain=excluded.source_domain,
  discovery_channel=excluded.discovery_channel,
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
  quotes=excluded.quotes`)
		if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
			return fmt.Errorf("record signal batch %d-%d: %w", start, end, err)
		}
	}
	return tx.Commit()
}

// RecordTrendSignalsBatch is the equivalent batched path for scanner trend
// membership persistence. first_observed_at remains immutable on conflict while
// observed_at advances, matching RecordTrendSignals.
func (s *Store) RecordTrendSignalsBatch(ctx context.Context, trendKey string, values []model.Signal, observedAt time.Time) error {
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

	ids := make([]string, 0, len(values))
	for _, signal := range values {
		if signal.ID != "" {
			ids = append(ids, signal.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for start := 0; start < len(ids); start += membershipWriteBatchSize {
		end := start + membershipWriteBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		var query strings.Builder
		query.WriteString("INSERT INTO trend_signal_memberships (trend_key, signal_id, first_observed_at, observed_at) VALUES ")
		args := make([]any, 0, len(chunk)*4)
		for i, signalID := range chunk {
			if i > 0 {
				query.WriteByte(',')
			}
			query.WriteString("(?, ?, ?, ?)")
			args = append(args, trendKey, signalID, stamp, stamp)
		}
		query.WriteString(" ON CONFLICT(trend_key, signal_id) DO UPDATE SET observed_at=excluded.observed_at")
		if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
			return fmt.Errorf("record trend membership batch %s %d-%d: %w", trendKey, start, end, err)
		}
	}
	return tx.Commit()
}
