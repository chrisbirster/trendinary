package scanner

import (
	"context"
	"fmt"
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/store"
)

type top20FixtureSource struct {
	values []model.Signal
}

func (s top20FixtureSource) Name() string { return "top20-fixture" }
func (s top20FixtureSource) Discover(context.Context) ([]model.Signal, error) {
	return append([]model.Signal(nil), s.values...), nil
}

func TestLowInformationBlueskyReplyIsSuppressed(t *testing.T) {
	values := []model.Signal{
		{ID: "bsky:1", Source: model.Source{Name: "Bluesky", Domain: "bsky.app"}, DiscoveryChannel: "bluesky", Text: "More or less, yes", Author: "a.example"},
		{ID: "bsky:2", Source: model.Source{Name: "Bluesky", Domain: "bsky.app"}, DiscoveryChannel: "bluesky", Text: "MacBook lid sensor apps are suddenly everywhere today", Author: "b.example"},
	}
	filtered := filterDiscoveryNoise(values)
	if filtered.Suppressed != 1 || len(filtered.Signals) != 1 || filtered.Signals[0].ID != "bsky:2" {
		t.Fatalf("unexpected filtered signals: %+v", filtered)
	}
}

func TestMirroredGitHubDescriptionsDoNotCountAsIndependentEvidence(t *testing.T) {
	description := "A long identical project description copied across two repositories so duplicate evidence can be discounted safely."
	values := []model.Signal{
		{ID: "github:1", Source: model.Source{Name: "GitHub", Domain: "github.com"}, DiscoveryChannel: "github", Author: "owner-one", Text: description},
		{ID: "github:2", Source: model.Source{Name: "GitHub", Domain: "github.com"}, DiscoveryChannel: "github", Author: "owner-two", Text: description},
	}
	independent := independentEvidenceSignals(values)
	if len(independent) != 1 {
		t.Fatalf("independent evidence = %d, want 1", len(independent))
	}
	all := provenanceSummary(values)
	if all.PublisherCount != 2 || all.PlatformCount != 1 {
		t.Fatalf("display provenance should preserve both accounts: %+v", all)
	}
	scoring := provenanceSummary(independent)
	if scoring.PublisherCount != 1 {
		t.Fatalf("scoring provenance should discount mirror: %+v", scoring)
	}
}

func TestProvenanceSeparatesPublisherPlatformAndDiscovery(t *testing.T) {
	values := []model.Signal{
		{ID: "github:1", Source: model.Source{Name: "GitHub", Domain: "github.com"}, DiscoveryChannel: "github", Author: "jlxc2001"},
		{ID: "github:2", Source: model.Source{Name: "GitHub", Domain: "github.com"}, DiscoveryChannel: "github", Author: "opensourcevillain"},
		{ID: "rss:1", Source: model.Source{Name: "WIRED · Top Stories", Domain: "wired.com"}, DiscoveryChannel: "rss"},
		{ID: "gdelt:1", Source: model.Source{Name: "WIRED · AI", Domain: "wired.com"}, DiscoveryChannel: "gdelt"},
	}
	got := provenanceSummary(values)
	if got.PublisherCount != 3 {
		t.Fatalf("publisher count = %d, want 3: %+v", got.PublisherCount, got)
	}
	if got.PlatformCount != 2 {
		t.Fatalf("platform count = %d, want 2: %+v", got.PlatformCount, got)
	}
	if len(got.DiscoveryChannels) != 3 {
		t.Fatalf("discovery channels = %+v", got.DiscoveryChannels)
	}
}

func TestChartCandidateCanBeEmergingBeforeStrictCorroboration(t *testing.T) {
	cluster := engine.Cluster{Signals: []model.Signal{{
		ID: "rss:1", Source: model.Source{Name: "ABC News", Domain: "abcnews.go.com"}, DiscoveryChannel: "rss",
		Title: "A developing story attracts unusual attention",
	}}}
	if !chartCandidateCluster(cluster) {
		t.Fatal("single substantive publisher should be chartable as emerging")
	}
	if candidateClusterV2(cluster) {
		t.Fatal("single publisher must not be treated as strictly corroborated")
	}
}

func TestGoogleTrendsHasDistinctPlatformProvenance(t *testing.T) {
	got := provenanceSummary([]model.Signal{{
		ID: "rss:google", Source: model.Source{Name: "Google Trends", Domain: "trends.google.com"}, DiscoveryChannel: "rss",
		Title: "example search",
	}})
	if got.PublisherCount != 1 || got.PlatformCount != 1 || len(got.Platforms) != 1 || got.Platforms[0] != "Google Trends" {
		t.Fatalf("unexpected Google Trends provenance: %+v", got)
	}
}

func TestStrongestClustersLimitsSinglePlatformFloodWhenAlternativesExist(t *testing.T) {
	clusters := make([]engine.Cluster, 0, 16)
	for i := 0; i < 8; i++ {
		clusters = append(clusters, engine.Cluster{Key: fmt.Sprintf("youtube-%02d", i), Signals: []model.Signal{{
			ID: fmt.Sprintf("youtube:%02d", i), Source: model.Source{Name: fmt.Sprintf("Channel %02d", i), Domain: "youtube.com"},
			DiscoveryChannel: "youtube", Author: fmt.Sprintf("Channel %02d", i), Title: fmt.Sprintf("YouTube topic %02d", i),
			Engagement: model.Engagement{Score: 100_000_000 - i},
		}}})
	}
	for i := 0; i < 8; i++ {
		clusters = append(clusters, engine.Cluster{Key: fmt.Sprintf("web-%02d", i), Signals: []model.Signal{{
			ID: fmt.Sprintf("rss:%02d", i), Source: model.Source{Name: fmt.Sprintf("Publisher %02d", i), Domain: fmt.Sprintf("publisher-%02d.example", i)},
			DiscoveryChannel: "rss", Title: fmt.Sprintf("Web topic %02d", i),
		}}})
	}

	got := strongestClusters(clusters, 8)
	if len(got) != 8 {
		t.Fatalf("shortlist length = %d, want 8", len(got))
	}
	youtube := 0
	for _, cluster := range got {
		if singlePlatformCandidateBucket(cluster) == "youtube" {
			youtube++
		}
	}
	if youtube != 2 {
		t.Fatalf("YouTube shortlist entries = %d, want 2 when alternatives exist", youtube)
	}
}

func TestStrongestClustersBackfillsSinglePlatformWhenChartWouldOtherwiseBeSparse(t *testing.T) {
	clusters := make([]engine.Cluster, 0, 8)
	for i := 0; i < 8; i++ {
		clusters = append(clusters, engine.Cluster{Key: fmt.Sprintf("youtube-%02d", i), Signals: []model.Signal{{
			ID: fmt.Sprintf("youtube:%02d", i), Source: model.Source{Name: fmt.Sprintf("Channel %02d", i), Domain: "youtube.com"},
			DiscoveryChannel: "youtube", Author: fmt.Sprintf("Channel %02d", i), Title: fmt.Sprintf("YouTube topic %02d", i),
			Engagement: model.Engagement{Score: 1000 + i},
		}}})
	}
	got := strongestClusters(clusters, 8)
	if len(got) != 8 {
		t.Fatalf("shortlist length = %d, want 8 after single-platform backfill", len(got))
	}
}

func TestScannerPublishesRanksOneThroughTwenty(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()
	memory := store.NewMemory()

	values := make([]model.Signal, 0, 20)
	for i := 1; i <= 20; i++ {
		values = append(values, model.Signal{
			ID:               fmt.Sprintf("rss:fixture-%02d", i),
			Source:           model.Source{Name: fmt.Sprintf("Publisher %02d", i), Domain: fmt.Sprintf("publisher-%02d.example", i)},
			DiscoveryChannel: "rss",
			Title:            fmt.Sprintf("Quasar%02d Zephyr%02d Nebula%02d Flux%02d", i, i, i, i),
			PublishedAt:      "2026-09-12T20:00:00Z",
		})
	}

	s := New(nil, nil, historical, memory, Config{
		PublishedTrendLimit: 20,
		ClusterThreshold:    0.95,
		SourceUniverse:      8,
	})
	result, err := s.RunWithSourcesV2(context.Background(), nil, []DiscoverySource{top20FixtureSource{values: values}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Trends != 20 {
		t.Fatalf("published trends = %d, want 20 (warnings=%v)", result.Trends, result.Warnings)
	}
	trends := memory.Trends()
	if len(trends) != 20 {
		t.Fatalf("memory chart length = %d, want 20", len(trends))
	}
	for i, trend := range trends {
		want := i + 1
		if trend.Rank != want {
			t.Fatalf("chart position %d has rank %d", want, trend.Rank)
		}
		if trend.ConfidenceTier == "" {
			t.Fatalf("chart position %d missing confidence tier", want)
		}
		if trend.Provenance.PublisherCount < 1 || trend.Provenance.PlatformCount < 1 {
			t.Fatalf("chart position %d missing provenance: %+v", want, trend.Provenance)
		}
	}
}
