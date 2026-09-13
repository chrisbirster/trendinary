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
	CursorName                   = "atproto-jetstream-v2-posts"
	postsCollection              = "app.bsky.feed.post"
	defaultHost                  = "https://jetstream.us-east.bsky.network"
	defaultStreamStaleAfter      = time.Minute
	freshStreamEventMaxAge       = 10 * time.Minute
	maxStreamWatchdogInterval    = 30 * time.Second
	minStreamWatchdogInterval    = 10 * time.Millisecond
	defaultCursorCheckpointEvery = 5 * time.Second
)

var (
	ErrFatal       = errors.New("trendinary jetstream fatal error")
	ErrStreamEnded = errors.New("trendinary jetstream stream ended")
)

type Config struct {
	Host                  string
	BatchSize             int
	Status                *runtimeinfo.Status
	APIKey                string
	StaleAfter            time.Duration
	CursorCheckpointEvery time.Duration
}

type Collector struct {
	history              *history.Store
	recent               *recent.Store
	config               Config
	lastCursorCheckpoint time.Time
}

func New(historical *history.Store, recentSignals *recent.Store, config Config) *Collector {
	if config.Host == "" {
		config.Host = defaultHost
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 128
	}
	if config.StaleAfter <= 0 {
		config.StaleAfter = defaultStreamStaleAfter
	}
	if config.CursorCheckpointEvery <= 0 {
		config.CursorCheckpointEvery = defaultCursorCheckpointEvery
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
		opts = append(opts,
			bskyjetstream.WithAPIKey(c.config.APIKey),
			bskyjetstream.WithAfterSeq(cursor),
		)
		return opts, "archive-replay"
	}
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

func watchdogInterval(staleAfter time.Duration) time.Duration {
	interval := staleAfter / 4
	if interval > maxStreamWatchdogInterval {
		interval = maxStreamWatchdogInterval
	}
	if interval < minStreamWatchdogInterval {
		interval = minStreamWatchdogInterval
	}
	return interval
}

func streamProgressed(snapshot runtimeinfo.StreamSnapshot, lastCursor uint64, lastEventAt time.Time) bool {
	return snapshot.LastCursor != lastCursor || snapshot.LastEventAt.After(lastEventAt)
}

func shouldDropResumeGap(mode, apiKey string, snapshot runtimeinfo.StreamSnapshot, now time.Time) bool {
	if mode != "live-resume" || strings.TrimSpace(apiKey) != "" {
		return false
	}
	if snapshot.LastEventAt.IsZero() {
		return true
	}
	return now.UTC().Sub(snapshot.LastEventAt) > freshStreamEventMaxAge
}

func resumeFreshnessExpired(mode, apiKey string, snapshot runtimeinfo.StreamSnapshot, connectedAt, now time.Time, grace time.Duration) bool {
	if !shouldDropResumeGap(mode, apiKey, snapshot, now) {
		return false
	}
	if grace <= 0 {
		grace = defaultStreamStaleAfter
	}
	return connectedAt.IsZero() || now.UTC().Sub(connectedAt.UTC()) >= grace
}

func recycleStalledSubscription(cancel context.CancelFunc, client *bskyjetstream.Client) {
	if cancel != nil {
		cancel()
	}
	if client != nil {
		_ = client.Close()
	}
}

// watchLiveProgress recycles a live subscription when it stops progressing. A
// credential-free cursor resume also has a freshness deadline: advancing through
// old history does not count as healthy forever. Authenticated archive replay is
// exempt so it can catch up the complete durable gap before cutting over live.
func (c *Collector) watchLiveProgress(ctx context.Context, cancel context.CancelFunc, client *bskyjetstream.Client, mode string, connectedAt time.Time, stop <-chan struct{}, stalled chan<- struct{}, done chan<- struct{}) {
	defer close(done)
	if c == nil || client == nil || c.config.Status == nil || c.config.StaleAfter <= 0 {
		return
	}
	initial := c.config.Status.Snapshot().Stream
	lastCursor := initial.LastCursor
	lastEventAt := initial.LastEventAt
	lastProgressAt := connectedAt

	ticker := time.NewTicker(watchdogInterval(c.config.StaleAfter))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case now := <-ticker.C:
			snapshot := c.config.Status.Snapshot().Stream
			if !snapshot.Connected {
				continue
			}
			if resumeFreshnessExpired(mode, c.config.APIKey, snapshot, connectedAt, now, c.config.StaleAfter) {
				slog.Warn("jetstream public resume is advancing stale history; abandoning gap and attaching at current live tip",
					"host", c.config.Host,
					"last_event_at", snapshot.LastEventAt,
					"last_cursor", snapshot.LastCursor,
					"connected_at", connectedAt,
				)
				select {
				case stalled <- struct{}{}:
				default:
				}
				recycleStalledSubscription(cancel, client)
				return
			}
			if streamProgressed(snapshot, lastCursor, lastEventAt) {
				lastCursor = snapshot.LastCursor
				if snapshot.LastEventAt.After(lastEventAt) {
					lastEventAt = snapshot.LastEventAt
				}
				lastProgressAt = now.UTC()
				continue
			}
			if now.UTC().Sub(lastProgressAt) < c.config.StaleAfter {
				continue
			}
			slog.Warn("jetstream stream stalled; recycling live subscription",
				"host", c.config.Host,
				"last_event_at", snapshot.LastEventAt,
				"last_cursor", snapshot.LastCursor,
				"stale_after", c.config.StaleAfter,
			)
			select {
			case stalled <- struct{}{}:
			default:
			}
			recycleStalledSubscription(cancel, client)
			return
		}
	}
}

// Run consumes app.bsky.feed.post commits until the context is cancelled or
// Jetstream reports a terminal failure. A configured API key uses authenticated
// archive replay from the durable cursor before cutting over to the live stream.
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
		streamCtx, streamCancel := context.WithCancel(ctx)
		connectedAt := time.Now().UTC()
		c.config.Status.StreamConnected(c.config.Host)
		if hasCursor {
			c.config.Status.StreamBatch(cursor, 0, time.Time{})
		}
		watchdogStop := make(chan struct{})
		watchdogStalled := make(chan struct{}, 1)
		watchdogDone := make(chan struct{})
		go c.watchLiveProgress(streamCtx, streamCancel, client, mode, connectedAt, watchdogStop, watchdogStalled, watchdogDone)

		restartAtLiveTip := false
		for batch, streamErr := range client.Events(streamCtx) {
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
					close(watchdogStop)
					<-watchdogDone
					streamCancel()
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
				close(watchdogStop)
				<-watchdogDone
				streamCancel()
				_ = client.Close()
				return err
			}
			cursor = batch.LastCursor()
			hasCursor = cursor > 0
		}

		if cursor > 0 && ctx.Err() == nil {
			if err := c.checkpointCursor(ctx, cursor, true); err != nil {
				close(watchdogStop)
				<-watchdogDone
				streamCancel()
				_ = client.Close()
				return err
			}
		}
		close(watchdogStop)
		<-watchdogDone
		streamCancel()
		_ = client.Close()

		stalled := false
		select {
		case <-watchdogStalled:
			stalled = true
		default:
		}

		if restartAtLiveTip {
			cursor = 0
			hasCursor = false
			opts, mode = c.subscriptionOptions(0, false)
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if stalled {
			snapshot := c.config.Status.Snapshot().Stream
			c.config.Status.StreamDisconnected(ErrStreamEnded, 0)
			if shouldDropResumeGap(mode, c.config.APIKey, snapshot, time.Now().UTC()) {
				slog.Warn("jetstream public cursor resume stalled on stale history; attaching at current live tip",
					"cursor", cursor,
					"last_event_at", snapshot.LastEventAt,
					"configure_archive_replay", "TRENDINARY_JETSTREAM_API_KEY",
				)
				cursor = 0
				hasCursor = false
			}
			opts, mode = c.subscriptionOptions(cursor, hasCursor)
			continue
		}
		return ErrStreamEnded
	}
}

// applyBatch folds Jetstream mutations into the bounded in-memory attention
// window. Raw posts are intentionally not durable; only scanner-selected trend
// evidence is written to Turso. The cursor is checkpointed every few seconds.
func (c *Collector) applyBatch(ctx context.Context, events []bskyjetstream.Event, cursor uint64) error {
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
	for _, id := range upsertIDs {
		item := upserts[id]
		c.recent.Upsert(item.signal, item.observedAt)
	}

	deleteIDs := make([]string, 0, len(deletes))
	for id := range deletes {
		deleteIDs = append(deleteIDs, id)
	}
	sort.Strings(deleteIDs)
	for _, id := range deleteIDs {
		c.recent.Delete(id)
	}

	if err := c.checkpointCursor(ctx, cursor, false); err != nil {
		return err
	}
	if c.config.Status != nil {
		c.config.Status.StreamBatch(cursor, len(events), latestObserved)
	}
	return nil
}

func (c *Collector) checkpointCursor(ctx context.Context, cursor uint64, force bool) error {
	if cursor == 0 {
		return nil
	}
	now := time.Now().UTC()
	if !force && !c.lastCursorCheckpoint.IsZero() && now.Sub(c.lastCursorCheckpoint) < c.config.CursorCheckpointEvery {
		return nil
	}
	if err := c.history.SaveCursor(ctx, CursorName, cursor); err != nil {
		return fmt.Errorf("save jetstream cursor: %w", err)
	}
	c.lastCursorCheckpoint = now
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
