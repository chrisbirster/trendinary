package jetstream

import (
	"context"
	"testing"
	"time"

	bskyjetstream "github.com/bluesky-social/jetstream"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/recent"
)

func TestApplyBatchFoldsCreateUpdateDeleteAndCursor(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	window := recent.New(100, time.Hour)
	collector := New(historical, window, Config{})
	ctx := context.Background()

	created := bskyjetstream.Event{
		DID: "did:plc:alice",
		Seq: 10,
		TimeUS: time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC).UnixMicro(),
		Kind: bskyjetstream.KindCommit,
		Commit: &bskyjetstream.Commit{
			Operation: bskyjetstream.OpCreate,
			Collection: postsCollection,
			Rkey: "abc",
			Record: map[string]any{
				"text": "A new protocol is suddenly everywhere",
				"createdAt": "2026-08-31T12:00:00Z",
			},
		},
	}
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

	updated := created
	updated.Seq = 11
	updated.Commit = &bskyjetstream.Commit{
		Operation: bskyjetstream.OpUpdate,
		Collection: postsCollection,
		Rkey: "abc",
		Record: map[string]any{
			"text": "The protocol is accelerating faster now",
			"createdAt": "2026-08-31T12:00:00Z",
		},
	}
	if err := collector.applyBatch(ctx, []bskyjetstream.Event{updated}, 11); err != nil {
		t.Fatal(err)
	}
	values = window.Recent(time.Time{})
	if len(values) != 1 || values[0].Text != "The protocol is accelerating faster now" {
		t.Fatalf("update was not folded: %+v", values)
	}

	deleted := created
	deleted.Seq = 12
	deleted.Commit = &bskyjetstream.Commit{
		Operation: bskyjetstream.OpDelete,
		Collection: postsCollection,
		Rkey: "abc",
	}
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
