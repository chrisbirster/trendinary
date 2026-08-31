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
	}); err != nil {
		t.Fatal(err)
	}

	snapshots, err := store.RecentSnapshots(ctx, "at-protocol", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(snapshots))
	}
	if snapshots[0].Score.Score != score.Score || snapshots[0].Lifecycle != "RISING" {
		t.Fatalf("unexpected snapshot: %+v", snapshots[0])
	}
	if !snapshots[0].ObservedAt.Equal(observed) {
		t.Fatalf("observed_at = %s, want %s", snapshots[0].ObservedAt, observed)
	}
}
