package scanner

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

func TestCandidateClusterRejectsWikipediaSingleton(t *testing.T) {
	cluster := engine.Cluster{Signals: []model.Signal{
		{
			ID:               "wikipedia:Andy_Ruiz_Jr.",
			Source:           model.Source{Name: "Wikipedia", Domain: "wikipedia.org"},
			DiscoveryChannel: "wikipedia",
			Title:            "Andy Ruiz Jr.",
			Engagement:       model.Engagement{Score: 5_000_000},
		},
	}}
	if candidateCluster(cluster) {
		t.Fatal("a Wikipedia pageview observation must not seed a public trend by itself")
	}
}

func TestCandidateClusterRejectsWikipediaPlusOnePublisher(t *testing.T) {
	cluster := engine.Cluster{Signals: []model.Signal{
		{
			ID:               "wikipedia:Andy_Ruiz_Jr.",
			Source:           model.Source{Name: "Wikipedia", Domain: "wikipedia.org"},
			DiscoveryChannel: "wikipedia",
			Title:            "Andy Ruiz Jr.",
			Engagement:       model.Engagement{Score: 5_000_000},
		},
		{
			ID:               "rss:boxing-story",
			Source:           model.Source{Name: "Example News", Domain: "example.com"},
			DiscoveryChannel: "rss",
			Title:            "Andy Ruiz Jr. returns to training",
		},
	}}
	if candidateCluster(cluster) {
		t.Fatal("context-only Wikipedia evidence plus one publisher is still only one independent candidate signal")
	}
}

func TestCandidateClusterRejectsDuplicateDiscoveryFromSamePublisher(t *testing.T) {
	cluster := engine.Cluster{Signals: []model.Signal{
		{
			ID:               "rss:story",
			Source:           model.Source{Name: "Example News", Domain: "example.com"},
			DiscoveryChannel: "rss",
			Title:            "A protocol launch accelerates",
		},
		{
			ID:               "gdelt:story",
			Source:           model.Source{Name: "Example News", Domain: "example.com"},
			DiscoveryChannel: "gdelt",
			Title:            "A protocol launch accelerates",
		},
	}}
	if candidateCluster(cluster) {
		t.Fatal("the same publisher discovered twice must not masquerade as independent corroboration")
	}
}

func TestCandidateClusterAcceptsIndependentPublishers(t *testing.T) {
	cluster := engine.Cluster{Signals: []model.Signal{
		{
			ID:               "rss:first",
			Source:           model.Source{Name: "First News", Domain: "first.example"},
			DiscoveryChannel: "rss",
			Title:            "A protocol launch accelerates",
		},
		{
			ID:               "gdelt:second",
			Source:           model.Source{Name: "Second News", Domain: "second.example"},
			DiscoveryChannel: "gdelt",
			Title:            "A protocol launch accelerates",
		},
	}}
	if !candidateCluster(cluster) {
		t.Fatal("two independent publisher domains should admit a candidate")
	}
}

func TestCandidateClusterAcceptsIndependentCommunityActors(t *testing.T) {
	cluster := engine.Cluster{Signals: []model.Signal{
		{
			ID:       "bsky:one",
			Source:   model.Source{Name: "Bluesky", Domain: "bsky.app"},
			AuthorID: "did:plc:one",
			Text:     "A strange new protocol is spreading",
		},
		{
			ID:       "bsky:two",
			Source:   model.Source{Name: "Bluesky", Domain: "bsky.app"},
			AuthorID: "did:plc:two",
			Text:     "That strange new protocol is spreading fast",
		},
	}}
	if !candidateCluster(cluster) {
		t.Fatal("two independent community actors should admit a candidate even on one network")
	}
}

func TestCandidateWeightIgnoresWikipediaPageviewMagnitude(t *testing.T) {
	base := engine.Cluster{Signals: []model.Signal{
		{
			ID:               "rss:first",
			Source:           model.Source{Name: "First News", Domain: "first.example"},
			DiscoveryChannel: "rss",
			Engagement:       model.Engagement{Score: 10},
		},
		{
			ID:               "rss:second",
			Source:           model.Source{Name: "Second News", Domain: "second.example"},
			DiscoveryChannel: "rss",
			Engagement:       model.Engagement{Score: 12},
		},
	}}
	withWikipedia := base
	withWikipedia.Signals = append(append([]model.Signal(nil), base.Signals...), model.Signal{
		ID:               "wikipedia:topic",
		Source:           model.Source{Name: "Wikipedia", Domain: "wikipedia.org"},
		DiscoveryChannel: "wikipedia",
		Engagement:       model.Engagement{Score: 20_000_000},
	})
	if got, want := candidateWeight(withWikipedia), candidateWeight(base); got != want {
		t.Fatalf("Wikipedia pageviews changed candidate priority: got %d want %d", got, want)
	}
}
