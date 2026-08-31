package history_test

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
)

func TestSignalAndSnapshotHistory(t *testing.T) {
	store, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.RecordSignals(ctx, []model.Signal{{
		ID: "hn:1",
		Source: model.Source{Name: "Hacker News", Domain: "news.ycombinator.com"},
		Title: "AT Protocol is moving",
		Engagement: model.Engagement{Score: 50, Replies: 12},
	}}); err != nil {
		t.Fatal(err)
	}

	score := engine.Score(engine.ScoreInput{
		Attention: 0.5, Velocity: 0.9, SourceBreadth: 0.8,
		CommunityBreadth: 0.7, Novelty: 0.8, Confidence: 0.9,
	})
	observed := time.Date(2026, 8, 30, 21, 0, 0, 0, time.UTC)
	if err := store.RecordSnapshot(ctx, history.Snapshot{
		TrendKey: "at-protocol", ObservedAt: observed, Lifecycle: "RISING", Score: score,
		Raw: history.RawMetrics{SignalCount: 3, SourceCount: 2, CommunityCount: 3, RawAttention: 100, RawEngagement: 77},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSnapshot(ctx, history.Snapshot{
		TrendKey: "at-protocol", ObservedAt: observed.Add(time.Hour), Lifecycle: "BREAKING", Score: score,
		Raw: history.RawMetrics{SignalCount: 5, SourceCount: 2, CommunityCount: 5, RawAttention: 200, RawEngagement: 140},
	}); err != nil {
		t.Fatal(err)
	}

	snapshots, err := store.RecentSnapshots(ctx, "at-protocol", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(snapshots))
	}
	if snapshots[0].Raw.RawAttention != 200 || snapshots[0].Lifecycle != "BREAKING" {
		t.Fatalf("unexpected newest snapshot: %+v", snapshots[0])
	}

	baseline, err := store.Baseline(ctx, "at-protocol", observed.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Observations != 2 {
		t.Fatalf("observations = %d, want 2", baseline.Observations)
	}
	if baseline.AverageAttention != 150 {
		t.Fatalf("average attention = %f, want 150", baseline.AverageAttention)
	}
	if baseline.Latest == nil || baseline.Latest.Raw.RawAttention != 200 {
		t.Fatalf("unexpected latest baseline snapshot: %+v", baseline.Latest)
	}
}

func TestStreamCursorRoundTrip(t *testing.T) {
	store, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if cursor, ok, err := store.Cursor(ctx, "atproto-jetstream"); err != nil {
		t.Fatal(err)
	} else if ok || cursor != 0 {
		t.Fatalf("unexpected initial cursor: %d %t", cursor, ok)
	}

	if err := store.SaveCursor(ctx, "atproto-jetstream", 123456); err != nil {
		t.Fatal(err)
	}
	cursor, ok, err := store.Cursor(ctx, "atproto-jetstream")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cursor != 123456 {
		t.Fatalf("cursor = %d, ok = %t", cursor, ok)
	}

	if err := store.SaveCursor(ctx, "atproto-jetstream", 123999); err != nil {
		t.Fatal(err)
	}
	cursor, ok, err = store.Cursor(ctx, "atproto-jetstream")
	if err != nil || !ok || cursor != 123999 {
		t.Fatalf("updated cursor = %d, ok = %t, err = %v", cursor, ok, err)
	}
}
