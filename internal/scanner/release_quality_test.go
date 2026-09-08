package scanner_test

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/scanner"
	"github.com/chrisbirster/trendinary/internal/store"
)

func TestReleaseGateUnrelatedWikipediaPagesNeverBecomeOneTrend(t *testing.T) {
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
			ID: "wikipedia:List_of_highest-grossing_films",
			Source: model.Source{Name: "Wikipedia", Domain: "wikipedia.org"},
			DiscoveryChannel: "wikipedia",
			Title: "List of highest-grossing films",
			PublishedAt: now.Add(-time.Minute).Format(time.RFC3339),
			Engagement: model.Engagement{Score: 8_000_000},
		},
		{
			ID: "wikipedia:List_of_Intel_codenames",
			Source: model.Source{Name: "Wikipedia", Domain: "wikipedia.org"},
			DiscoveryChannel: "wikipedia",
			Title: "List of Intel codenames",
			PublishedAt: now.Add(-time.Minute).Format(time.RFC3339),
			Engagement: model.Engagement{Score: 7_000_000},
		},
	}, now)

	s := scanner.New(nil, nil, historical, memory, scanner.Config{PublishedTrendLimit: 20, ClusterThreshold: 0.35, SourceUniverse: 8})
	result, err := s.RunWithRecent(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends != 0 || len(memory.Trends()) != 0 {
		t.Fatalf("unrelated Wikipedia pages leaked into leaderboard: result=%+v trends=%+v", result, memory.Trends())
	}
}

func TestReleaseGateWikipediaPlusOnePublisherStillDoesNotQualify(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	memory := store.NewMemory()
	window := recent.New(100, time.Hour)
	now := time.Now().UTC()
	window.UpsertMany([]model.Signal{
		{ID: "wikipedia:Andy_Ruiz_Jr", Source: model.Source{Name: "Wikipedia", Domain: "wikipedia.org"}, DiscoveryChannel: "wikipedia", Title: "Andy Ruiz Jr.", Engagement: model.Engagement{Score: 5_000_000}},
		{ID: "rss:boxing", Source: model.Source{Name: "Boxing Daily", Domain: "boxing.example"}, DiscoveryChannel: "rss", Title: "Andy Ruiz Jr. returns to training"},
	}, now)

	s := scanner.New(nil, nil, historical, memory, scanner.Config{PublishedTrendLimit: 20, ClusterThreshold: 0.35, SourceUniverse: 8})
	result, err := s.RunWithRecent(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends != 0 || len(memory.Trends()) != 0 {
		t.Fatalf("Wikipedia plus one publisher must remain below public trend threshold: %+v", memory.Trends())
	}
}

func TestReleaseGateTwoIndependentPublishersProduceOneTrend(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	memory := store.NewMemory()
	window := recent.New(100, time.Hour)
	now := time.Now().UTC()
	window.UpsertMany([]model.Signal{
		{ID: "rss:first", Source: model.Source{Name: "First News", Domain: "first.example"}, DiscoveryChannel: "rss", Title: "Atlas browser agent launches today", PublishedAt: now.Add(-2 * time.Minute).Format(time.RFC3339)},
		{ID: "gdelt:second", Source: model.Source{Name: "Second News", Domain: "second.example"}, DiscoveryChannel: "gdelt", Title: "Atlas browser agent launches today", PublishedAt: now.Add(-time.Minute).Format(time.RFC3339)},
	}, now)

	s := scanner.New(nil, nil, historical, memory, scanner.Config{PublishedTrendLimit: 20, ClusterThreshold: 0.35, SourceUniverse: 8})
	result, err := s.RunWithRecent(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends != 1 {
		t.Fatalf("expected one corroborated trend, got %+v", result)
	}
	trends := memory.Trends()
	if len(trends) != 1 || len(trends[0].Sources) != 2 {
		t.Fatalf("expected one trend with two publisher sources, got %+v", trends)
	}
}

func TestReleaseGateDuplicateDiscoveryFromSamePublisherDoesNotQualify(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	memory := store.NewMemory()
	window := recent.New(100, time.Hour)
	now := time.Now().UTC()
	window.UpsertMany([]model.Signal{
		{ID: "rss:story", Source: model.Source{Name: "Example News", Domain: "example.com"}, DiscoveryChannel: "rss", Title: "Protocol launch accelerates"},
		{ID: "gdelt:story", Source: model.Source{Name: "Example News", Domain: "example.com"}, DiscoveryChannel: "gdelt", Title: "Protocol launch accelerates"},
	}, now)

	s := scanner.New(nil, nil, historical, memory, scanner.Config{PublishedTrendLimit: 20, ClusterThreshold: 0.35, SourceUniverse: 8})
	result, err := s.RunWithRecent(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends != 0 || len(memory.Trends()) != 0 {
		t.Fatalf("same publisher discovered twice must not become corroboration: %+v", memory.Trends())
	}
}
