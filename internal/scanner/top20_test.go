package scanner

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

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
