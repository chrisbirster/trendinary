package scanner_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/scanner"
	"github.com/chrisbirster/trendinary/internal/store"
)

type failingHN struct{}

func (failingHN) Top(context.Context, int) ([]hackernews.Item, error) {
	return nil, errors.New("upstream unavailable")
}

func TestRunWithRecentDiscoversTrendWithoutHackerNews(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	memory := store.NewMemory()
	window := recent.New(100, time.Hour)
	now := time.Now().UTC()
	window.UpsertMany([]model.Signal{
		{
			ID: "bsky:at://did:plc:a/app.bsky.feed.post/1",
			Source: model.Source{Name: "Bluesky", Domain: "bsky.app", URL: "https://bsky.app/"},
			Text: "SolidJS native apps are accelerating quickly",
			Author: "did:plc:a",
			PublishedAt: now.Add(-2 * time.Minute).Format(time.RFC3339),
		},
		{
			ID: "bsky:at://did:plc:b/app.bsky.feed.post/2",
			Source: model.Source{Name: "Bluesky", Domain: "bsky.app", URL: "https://bsky.app/"},
			Text: "SolidJS native app development is accelerating",
			Author: "did:plc:b",
			PublishedAt: now.Add(-time.Minute).Format(time.RFC3339),
		},
	}, now)

	s := scanner.New(nil, nil, historical, memory, scanner.Config{
		PublishedTrendLimit: 10,
		ClusterThreshold: 0.35,
		SourceUniverse: 2,
	})
	result, err := s.RunWithRecent(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends == 0 || result.Signals < 2 {
		t.Fatalf("unexpected stream-only scan: %+v", result)
	}
	trends := memory.Trends()
	if len(trends) == 0 || len(trends[0].Sources) != 1 || trends[0].Sources[0].Domain != "bsky.app" {
		t.Fatalf("unexpected trend sources: %+v", trends)
	}
}

func TestRunWithRecentRejectsSingleAuthorStreamClusterAndPreservesLastGoodView(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	memory := store.NewMemory()
	before := memory.Trends()
	window := recent.New(100, time.Hour)
	window.Upsert(model.Signal{
		ID: "bsky:at://did:plc:a/app.bsky.feed.post/1",
		Source: model.Source{Name: "Bluesky", Domain: "bsky.app"},
		Text: "A strange new internet meme is spreading",
		Author: "did:plc:a",
	}, time.Now().UTC())

	s := scanner.New(nil, nil, historical, memory, scanner.Config{PublishedTrendLimit: 10})
	result, err := s.RunWithRecent(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends != 0 || result.Clusters != 0 || len(result.Warnings) == 0 {
		t.Fatalf("singleton stream post should not become a trend: %+v", result)
	}
	after := memory.Trends()
	if len(after) != len(before) || len(after) == 0 || after[0].Slug != before[0].Slug {
		t.Fatalf("last good trend view should be preserved; before=%+v after=%+v", before, after)
	}
}

func TestRunWithRecentSurvivesHackerNewsFailure(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	memory := store.NewMemory()
	window := recent.New(100, time.Hour)
	now := time.Now().UTC()
	window.UpsertMany([]model.Signal{
		{
			ID: "bsky:at://did:plc:a/app.bsky.feed.post/1",
			Source: model.Source{Name: "Bluesky", Domain: "bsky.app"},
			Text: "A strange new internet meme is spreading quickly",
			Author: "did:plc:a",
		},
		{
			ID: "bsky:at://did:plc:b/app.bsky.feed.post/2",
			Source: model.Source{Name: "Bluesky", Domain: "bsky.app"},
			Text: "The strange new internet meme is spreading",
			Author: "did:plc:b",
		},
	}, now)

	s := scanner.New(failingHN{}, nil, historical, memory, scanner.Config{PublishedTrendLimit: 10, ClusterThreshold: 0.35})
	result, err := s.RunWithRecent(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends == 0 || len(result.Warnings) == 0 {
		t.Fatalf("expected live trend plus HN warning: %+v", result)
	}
}
