package jetstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	bskyjetstream "github.com/bluesky-social/jetstream"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/runtimeinfo"
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
	Status    *runtimeinfo.Status
	APIKey    string
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
	config.APIKey = strings.TrimSpace(config.APIKey)
	if config.APIKey == "" {
		config.APIKey = strings.TrimSpace(os.Getenv("TRENDINARY_JETSTREAM_API_KEY"))
	}
	return &Collector{history: historical, recent: recentSignals, config: config}
}

func (c *Collector) Host() string {
	return c.config.Host
}

func (c *Collector) subscriptionOptions(cursor uint64, hasCursor bool) ([]bskyjetstream.Option, string) {
	opts := []bskyjetstream.Option{
		bskyjetstream.WithKinds([]bskyjetstream.Kind{bskyjetstream.KindCommit}),
		bskyjetstream.WithCollection(postsCollection),
		bskyjetstream.WithBatchSize(c.config.BatchSize),
	}
	if !hasCursor {
		return opts, "live"
	}
	if c.config.APIKey != "" {
		// Archive replay requires a bearer API key. The jetstream client scopes
		// this secret to archive XRPC/download requests and never sends it to the
		// public live WebSocket.
		opts = append(opts,
			bskyjetstream.WithAPIKey(c.config.APIKey),
			bskyjetstream.WithAfterSeq(cursor),
		)
		return opts, "archive-replay"
	}
	// A persisted cursor must never make the public live stream unavailable.
	// Without an archive credential, first attempt to resume the public live
	// tail from the durable cursor. If the cursor has fallen below the server's
	// bounded lookback floor, Run deliberately drops the gap and attaches at the
	// current live tip instead of leaving discovery offline.
	opts = append(opts, bskyjetstream.WithLiveCursor(cursor))
	return opts, "live-resume"
}

func liveCursorTooOld(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "live cursor too old") || strings.Contains(message, "cursor too old")
}

// Run consumes app.bsky.feed.post commits until the context is cancelled or
// Jetstream reports a terminal failure. When a durable cursor exists and an
// archive API key is configured, the v2 client replays from that sequence and
// then cuts over to live. Without a key it first attempts a public live resume;
// if that cursor is outside the server's lookback window, it deliberately
// attaches at the current live tip so missing archive credentials cannot keep
// continuous discovery offline.
func (c *Collector) Run(ctx context.Context) error {
	if c.history == nil || c.recent == nil {
		return fmt.Errorf("jetstream collector dependencies are incomplete")
	}

	c.config.Status.StreamConnecting(c.config.Host)
	cursor, hasCursor, err := c.history.Cursor(ctx, CursorName)
	if err != nil {
		return fmt.Errorf("load jetstream cursor: %w", err)
	}

	opts, mode := c.subscriptionOptions(cursor, hasCursor)
	for {
		if mode == "live-resume" {
			slog.Warn("jetstream archive replay disabled; resuming public live tail from cursor",
				"cursor", cursor,
				"configure", "TRENDINARY_JETSTREAM_API_KEY",
			)
		} else {
			slog.Info("jetstream subscription configured", "mode", mode, "has_cursor", hasCursor)
		}

		client, err := bskyjetstream.Subscribe(c.config.Host, opts...)
		if err != nil {
			return fmt.Errorf("subscribe jetstream: %w", err)
		}
		c.config.Status.StreamConnected(c.config.Host)
		if hasCursor {
			c.config.Status.StreamBatch(cursor, 0, time.Time{})
		}

		restartAtLiveTip := false
		for batch, streamErr := range client.Events(ctx) {
			if streamErr != nil {
				if errors.Is(streamErr, bskyjetstream.ErrFatal) {
					if mode == "live-resume" && liveCursorTooOld(streamErr) {
						slog.Warn("jetstream live cursor is outside server lookback; dropping unreplayable gap and attaching at current live tip",
							"cursor", cursor,
							"configure_archive_replay", "TRENDINARY_JETSTREAM_API_KEY",
						)
						c.config.Status.StreamDisconnected(streamErr, 0)
						restartAtLiveTip = true
						break
					}
					_ = client.Close()
					return fmt.Errorf("%w: %v", ErrFatal, streamErr)
				}
				slog.Warn("recoverable jetstream error", "error", streamErr)
				c.config.Status.StreamDisconnected(streamErr, 0)
				continue
			}
			if batch == nil || len(batch.Events()) == 0 {
				continue
			}
			if err := c.applyBatch(ctx, batch.Events(), batch.LastCursor()); err != nil {
				_ = client.Close()
				return err
			}
		}
		_ = client.Close()

		if restartAtLiveTip {
			// No archive credential means the missing interval cannot be recovered.
			// Omit the stale cursor on the next subscription so Jetstream starts at
			// the current tip. The first delivered batch advances the durable cursor
			// normally, so subsequent restarts can resume from fresh state.
			cursor = 0
			hasCursor = false
			opts, mode = c.subscriptionOptions(0, false)
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrStreamEnded
	}
}

// applyBatch folds one Jetstream batch into durable application state before
// advancing the cursor. All operations are idempotent, so replay after a crash
// is safe if the process dies before SaveCursor succeeds.
func (c *Collector) applyBatch(ctx context.Context, events []bskyjetstream.Event, cursor uint64) error {
	// A batch can contain multiple mutations for the same record. Fold in wire
	// order so the last mutation wins before grouped durable writes are issued.
	upserts := make(map[string]observedSignal)
	deletes := make(map[string]struct{})

	latestObserved := time.Time{}
	for _, event := range events {
		if event.Kind != bskyjetstream.KindCommit || event.Commit == nil || event.Commit.Collection != postsCollection {
			continue
		}
		observed := eventTime(event)
		if observed.After(latestObserved) {
			latestObserved = observed
		}
		id := signalID(event.DID, event.Commit.Rkey)
		if id == "" {
			continue
		}

		if event.Commit.Operation == bskyjetstream.OpDelete {
			delete(upserts, id)
			deletes[id] = struct{}{}
			continue
		}

		signal, ok := normalizePost(event)
		if !ok {
			// An update can make a previously useful text post unclusterable.
			// Removing it keeps the live window consistent with upstream state.
			if event.Commit.Operation == bskyjetstream.OpUpdate {
				delete(upserts, id)
				deletes[id] = struct{}{}
			}
			continue
		}
		delete(deletes, id)
		upserts[id] = observedSignal{signal: signal, observedAt: observed}
	}

	upsertIDs := make([]string, 0, len(upserts))
	for id := range upserts {
		upsertIDs = append(upsertIDs, id)
	}
	sort.Strings(upsertIDs)
	values := make([]model.Signal, 0, len(upsertIDs))
	for _, id := range upsertIDs {
		values = append(values, upserts[id].signal)
	}
	if err := c.history.RecordSignals(ctx, values); err != nil {
		return fmt.Errorf("persist jetstream signals: %w", err)
	}

	deleteIDs := make([]string, 0, len(deletes))
	for id := range deletes {
		deleteIDs = append(deleteIDs, id)
	}
	sort.Strings(deleteIDs)
	if err := c.history.DeleteSignals(ctx, deleteIDs); err != nil {
		return fmt.Errorf("persist jetstream deletes: %w", err)
	}

	for _, id := range upsertIDs {
		item := upserts[id]
		c.recent.Upsert(item.signal, item.observedAt)
	}
	for _, id := range deleteIDs {
		c.recent.Delete(id)
	}

	if cursor > 0 {
		if err := c.history.SaveCursor(ctx, CursorName, cursor); err != nil {
			return fmt.Errorf("save jetstream cursor: %w", err)
		}
	}
	c.config.Status.StreamBatch(cursor, len(events), latestObserved)
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
		AuthorID:    event.DID,
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
