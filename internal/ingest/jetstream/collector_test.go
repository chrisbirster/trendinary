package jetstream

import (
	"context"
	"testing"
	"time"

	bskyjetstream "github.com/bluesky-social/jetstream"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/recent"
)

func TestNewLoadsJetstreamAPIKeyFromEnvironment(t *testing.T) {
	t.Setenv("TRENDINARY_JETSTREAM_API_KEY", "  archive-secret  ")
	collector := New(nil, nil, Config{})
	if collector.config.APIKey != "archive-secret" {
		t.Fatalf("api key = %q, want trimmed configured key", collector.config.APIKey)
	}
}

func TestSubscriptionModeFallsBackToPublicLiveResumeWithoutAPIKey(t *testing.T) {
	t.Setenv("TRENDINARY_JETSTREAM_API_KEY", "")
	collector := New(nil, nil, Config{})
	opts, mode := collector.subscriptionOptions(1234, true)
	if mode != "live-resume" {
		t.Fatalf("mode = %q, want live-resume", mode)
	}
	if len(opts) != 4 {
		t.Fatalf("options = %d, want base options plus live cursor", len(opts))
	}
}

func TestSubscriptionModeUsesAuthenticatedArchiveReplayWhenConfigured(t *testing.T) {
	collector := New(nil, nil, Config{APIKey: "archive-secret"})
	opts, mode := collector.subscriptionOptions(1234, true)
	if mode != "archive-replay" {
		t.Fatalf("mode = %q, want archive-replay", mode)
	}
	if len(opts) != 5 {
		t.Fatalf("options = %d, want base options plus api key and replay cursor", len(opts))
	}
}

func TestApplyBatchFoldsCreateUpdateDeleteAndCursor(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	window := recent.New(100, time.Hour)
	collector := New(historical, window, Config{})
	ctx := context.Background()

	created := testPostEvent(10, bskyjetstream.OpCreate, "A new protocol is suddenly everywhere")
	if err := collector.applyBatch(ctx, []bskyjetstream.Event{created}, 10); err != nil {
		t.Fatal(err)
	}
	if window.Len() != 1 {
		t.Fatalf("recent len = %d, want 1", window.Len())
	}
	cursor, ok, err := historical.Cursor(ctx, CursorName)
	if err != nil || !ok || cursor != 10 {
		t.Fatalf("cursor = %d ok=%v err=%v", cursor, ok, err)
	}
	values := window.Recent(time.Time{})
	if len(values) != 1 || values[0].Author != "did:plc:alice" || values[0].Source.Domain != "bsky.app" {
		t.Fatalf("unexpected normalized signal: %+v", values)
	}

	updated := testPostEvent(11, bskyjetstream.OpUpdate, "The protocol is accelerating faster now")
	if err := collector.applyBatch(ctx, []bskyjetstream.Event{updated}, 11); err != nil {
		t.Fatal(err)
	}
	values = window.Recent(time.Time{})
	if len(values) != 1 || values[0].Text != "The protocol is accelerating faster now" {
		t.Fatalf("update was not folded: %+v", values)
	}

	deleted := testPostEvent(12, bskyjetstream.OpDelete, "")
	if err := collector.applyBatch(ctx, []bskyjetstream.Event{deleted}, 12); err != nil {
		t.Fatal(err)
	}
	if window.Len() != 0 {
		t.Fatalf("recent len after delete = %d, want 0", window.Len())
	}
	cursor, ok, err = historical.Cursor(ctx, CursorName)
	if err != nil || !ok || cursor != 12 {
		t.Fatalf("cursor after delete = %d ok=%v err=%v", cursor, ok, err)
	}
}

func TestApplyBatchLastMutationWins(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()
	window := recent.New(100, time.Hour)
	collector := New(historical, window, Config{})
	ctx := context.Background()

	deleted := testPostEvent(20, bskyjetstream.OpDelete, "")
	recreated := testPostEvent(21, bskyjetstream.OpCreate, "Recreated post should survive")
	if err := collector.applyBatch(ctx, []bskyjetstream.Event{deleted, recreated}, 21); err != nil {
		t.Fatal(err)
	}
	values := window.Recent(time.Time{})
	if len(values) != 1 || values[0].Text != "Recreated post should survive" {
		t.Fatalf("delete then recreate should keep final record: %+v", values)
	}

	updated := testPostEvent(22, bskyjetstream.OpUpdate, "This should be deleted")
	deletedLast := testPostEvent(23, bskyjetstream.OpDelete, "")
	if err := collector.applyBatch(ctx, []bskyjetstream.Event{updated, deletedLast}, 23); err != nil {
		t.Fatal(err)
	}
	if window.Len() != 0 {
		t.Fatalf("update then delete should remove final record, len=%d", window.Len())
	}
}

func TestNormalizePostUsesStableATURIIdentity(t *testing.T) {
	event := bskyjetstream.Event{
		DID: "did:plc:test",
		Kind: bskyjetstream.KindCommit,
		Commit: &bskyjetstream.Commit{
			Operation: bskyjetstream.OpCreate,
			Collection: postsCollection,
			Rkey: "xyz",
			Record: map[string]any{
				"text": "hello trendinary",
				"createdAt": "2026-08-31T12:30:00Z",
			},
		},
	}
	signal, ok := normalizePost(event)
	if !ok {
		t.Fatal("expected post to normalize")
	}
	wantID := "bsky:at://did:plc:test/app.bsky.feed.post/xyz"
	if signal.ID != wantID {
		t.Fatalf("id = %q, want %q", signal.ID, wantID)
	}
	if signal.PublishedAt != "2026-08-31T12:30:00Z" {
		t.Fatalf("published_at = %q", signal.PublishedAt)
	}
}

func testPostEvent(seq uint64, operation bskyjetstream.Operation, text string) bskyjetstream.Event {
	eventAt := time.Now().UTC().Add(time.Duration(seq) * time.Millisecond)
	record := map[string]any(nil)
	if operation != bskyjetstream.OpDelete {
		record = map[string]any{
			"text": text,
			"createdAt": eventAt.Format(time.RFC3339Nano),
		}
	}
	return bskyjetstream.Event{
		DID: "did:plc:alice",
		Seq: seq,
		TimeUS: eventAt.UnixMicro(),
		Kind: bskyjetstream.KindCommit,
		Commit: &bskyjetstream.Commit{
			Operation: operation,
			Collection: postsCollection,
			Rkey: "abc",
			Record: record,
		},
	}
}
