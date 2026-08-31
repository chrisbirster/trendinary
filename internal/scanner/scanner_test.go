package scanner_test

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	"github.com/chrisbirster/trendinary/internal/scanner"
	"github.com/chrisbirster/trendinary/internal/store"
)

type fakeHN struct{}

func (fakeHN) Top(context.Context, int) ([]hackernews.Item, error) {
	return []hackernews.Item{
		{ID: 1, By: "alice", Score: 120, Descendants: 40, Time: 1788148800, Title: "AT Protocol apps are growing"},
		{ID: 2, By: "bob", Score: 90, Descendants: 20, Time: 1788148860, Title: "AT Protocol ecosystem is growing"},
	}, nil
}

type fakeBluesky struct{}

func (fakeBluesky) Search(context.Context, string, int) (bluesky.SearchResponse, error) {
	return bluesky.SearchResponse{Posts: []bluesky.Post{{
		URI: "at://did:plc:test/app.bsky.feed.post/abc",
		Author: bluesky.Author{Handle: "example.test"},
		Record: bluesky.Record{Text: "AT Protocol apps are suddenly everywhere", CreatedAt: "2026-08-31T09:10:00Z"},
		LikeCount: 50,
		RepostCount: 12,
	}}}, nil
}

func TestRunOncePublishesAndPersistsTrends(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	memory := store.NewMemory()
	s := scanner.New(fakeHN{}, fakeBluesky{}, historical, memory, scanner.Config{
		HackerNewsLimit: 10,
		EnrichClusters: 2,
		BlueskyLimit: 5,
		PublishedTrendLimit: 10,
		ClusterThreshold: 0.4,
		SourceUniverse: 2,
	})

	result, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends == 0 || result.Signals < 3 {
		t.Fatalf("unexpected scan result: %+v", result)
	}

	trends := memory.Trends()
	if len(trends) == 0 {
		t.Fatal("scanner did not publish trends")
	}
	if trends[0].Rank != 1 || trends[0].Score <= 0 {
		t.Fatalf("unexpected top trend: %+v", trends[0])
	}
	if len(trends[0].Sources) < 2 {
		t.Fatalf("expected cross-source enrichment, got %+v", trends[0].Sources)
	}

	snapshots, err := historical.RecentSnapshots(context.Background(), trends[0].Slug, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(snapshots))
	}
	if snapshots[0].Raw.SignalCount < 3 {
		t.Fatalf("unexpected raw metrics: %+v", snapshots[0].Raw)
	}

	// A second observation should establish a baseline and produce a non-NEW change label.
	time.Sleep(time.Millisecond)
	if _, err := s.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	updated := memory.Trends()
	if updated[0].Change == "NEW" {
		t.Fatalf("expected baseline-aware change after second scan: %+v", updated[0])
	}
}
