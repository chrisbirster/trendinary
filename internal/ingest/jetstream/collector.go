package jetstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	bskyjetstream "github.com/bluesky-social/jetstream"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
)

const (
	CursorName      = "atproto-jetstream-v2-posts"
	postsCollection = "app.bsky.feed.post"
	defaultHost     = "https://jetstream.us-east.bsky.network"
)

var (
	ErrFatal       = errors.New("trendinary jetstream fatal error")
	ErrStreamEnded = errors.New("trendinary jetstream stream ended")
)

type Config struct {
	Host      string
	BatchSize int
}

type Collector struct {
	history *history.Store
	recent  *recent.Store
	config  Config
}

func New(historical *history.Store, recentSignals *recent.Store, config Config) *Collector {
	if config.Host == "" {
		config.Host = defaultHost
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 128
	}
	return &Collector{history: historical, recent: recentSignals, config: config}
}

// Run consumes app.bsky.feed.post commits until the context is cancelled or
// Jetstream reports a terminal failure. When a durable cursor exists, the v2
// client replays from that sequence and then cuts over to the live tail.
func (c *Collector) Run(ctx context.Context) error {
	if c.history == nil || c.recent == nil {
		return fmt.Errorf("jetstream collector dependencies are incomplete")
	}

	cursor, hasCursor, err := c.history.Cursor(ctx, CursorName)
	if err != nil {
		return fmt.Errorf("load jetstream cursor: %w", err)
	}

	opts := []bskyjetstream.Option{
		bskyjetstream.WithKinds([]bskyjetstream.Kind{bskyjetstream.KindCommit}),
		bskyjetstream.WithCollection(postsCollection),
		bskyjetstream.WithBatchSize(c.config.BatchSize),
	}
	if hasCursor {
		opts = append(opts, bskyjetstream.WithAfterSeq(cursor))
	}

	client, err := bskyjetstream.Subscribe(c.config.Host, opts...)
	if err != nil {
		return fmt.Errorf("subscribe jetstream: %w", err)
	}
	defer client.Close()

	for batch, streamErr := range client.Events(ctx) {
		if streamErr != nil {
			if errors.Is(streamErr, bskyjetstream.ErrFatal) {
				return fmt.Errorf("%w: %v", ErrFatal, streamErr)
			}
			slog.Warn("recoverable jetstream error", "error", streamErr)
			continue
		}
		if batch == nil || len(batch.Events()) == 0 {
			continue
		}
		if err := c.applyBatch(ctx, batch.Events(), batch.LastCursor()); err != nil {
			return err
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	return ErrStreamEnded
}

// applyBatch folds one Jetstream batch into durable application state before
// advancing the cursor. All operations are idempotent, so replay after a crash
// is safe if the process dies before SaveCursor succeeds.
func (c *Collector) applyBatch(ctx context.Context, events []bskyjetstream.Event, cursor uint64) error {
	upserts := make([]observedSignal, 0, len(events))
	deletes := make([]string, 0)

	for _, event := range events {
		if event.Kind != bskyjetstream.KindCommit || event.Commit == nil || event.Commit.Collection != postsCollection {
			continue
		}
		id := signalID(event.DID, event.Commit.Rkey)
		if id == "" {
			continue
		}

		if event.Commit.Operation == bskyjetstream.OpDelete {
			deletes = append(deletes, id)
			continue
		}

		signal, ok := normalizePost(event)
		if !ok {
			// An update can make a previously useful text post unclusterable.
			// Removing it keeps the live window consistent with upstream state.
			if event.Commit.Operation == bskyjetstream.OpUpdate {
				deletes = append(deletes, id)
			}
			continue
		}
		upserts = append(upserts, observedSignal{signal: signal, observedAt: eventTime(event)})
	}

	if len(upserts) > 0 {
		values := make([]model.Signal, 0, len(upserts))
		for _, item := range upserts {
			values = append(values, item.signal)
		}
		if err := c.history.RecordSignals(ctx, values); err != nil {
			return fmt.Errorf("persist jetstream signals: %w", err)
		}
	}
	if err := c.history.DeleteSignals(ctx, deletes); err != nil {
		return fmt.Errorf("persist jetstream deletes: %w", err)
	}

	for _, item := range upserts {
		c.recent.Upsert(item.signal, item.observedAt)
	}
	for _, id := range deletes {
		c.recent.Delete(id)
	}

	if cursor > 0 {
		if err := c.history.SaveCursor(ctx, CursorName, cursor); err != nil {
			return fmt.Errorf("save jetstream cursor: %w", err)
		}
	}
	return nil
}

type observedSignal struct {
	signal     model.Signal
	observedAt time.Time
}

func normalizePost(event bskyjetstream.Event) (model.Signal, bool) {
	if event.Commit == nil || event.Commit.Record == nil {
		return model.Signal{}, false
	}
	text, _ := event.Commit.Record["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		return model.Signal{}, false
	}
	createdAt, _ := event.Commit.Record["createdAt"].(string)

	uri := postURI(event.DID, event.Commit.Rkey)
	if uri == "" {
		return model.Signal{}, false
	}
	return model.Signal{
		ID: "bsky:" + uri,
		Source: model.Source{
			Name:   "Bluesky",
			Domain: "bsky.app",
			URL:    "https://bsky.app/",
		},
		Text:        text,
		URL:         fmt.Sprintf("https://bsky.app/profile/%s/post/%s", event.DID, event.Commit.Rkey),
		Author:      event.DID,
		PublishedAt: createdAt,
	}, true
}

func signalID(did, rkey string) string {
	uri := postURI(did, rkey)
	if uri == "" {
		return ""
	}
	return "bsky:" + uri
}

func postURI(did, rkey string) string {
	did = strings.TrimSpace(did)
	rkey = strings.TrimSpace(rkey)
	if did == "" || rkey == "" {
		return ""
	}
	return fmt.Sprintf("at://%s/%s/%s", did, postsCollection, rkey)
}

func eventTime(event bskyjetstream.Event) time.Time {
	if event.TimeUS > 0 {
		return time.UnixMicro(event.TimeUS).UTC()
	}
	return time.Now().UTC()
}
