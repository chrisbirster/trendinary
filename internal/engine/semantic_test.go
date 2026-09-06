package engine_test

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

func TestClusterSignalsV2MergesDifferentlyWordedCrossPublisherEvent(t *testing.T) {
	values := []model.Signal{
		{
			ID:          "hn:audacity",
			Source:      model.Source{Name: "Hacker News", Domain: "news.ycombinator.com"},
			Title:       "Audacity 4.0 released with redesigned interface",
			PublishedAt: "2026-09-03T10:00:00Z",
		},
		{
			ID:          "wired:audacity",
			Source:      model.Source{Name: "WIRED", Domain: "wired.com"},
			Title:       "Audacity launches major version 4 update",
			PublishedAt: "2026-09-03T11:10:00Z",
		},
	}
	clusters := engine.ClusterSignalsV2(values, 0.42)
	if len(clusters) != 1 || len(clusters[0].Signals) != 2 {
		t.Fatalf("clusters = %+v, want one two-signal event", clusters)
	}
}

func TestClusterSignalsV2DoesNotMergeUnrelatedStoriesAboutSameEntity(t *testing.T) {
	values := []model.Signal{
		{
			ID:          "a",
			Title:       "OpenAI launches Atlas browser agent",
			PublishedAt: "2026-09-03T10:00:00Z",
		},
		{
			ID:          "b",
			Title:       "OpenAI appoints new finance chief",
			PublishedAt: "2026-09-03T11:00:00Z",
		},
	}
	clusters := engine.ClusterSignalsV2(values, 0.42)
	if len(clusters) != 2 {
		t.Fatalf("same entity but different events merged: %+v", clusters)
	}
}

func TestClusterSignalsV2DoesNotMergeWikipediaTopPages(t *testing.T) {
	values := []model.Signal{
		{ID: "w1", Title: "Killing of the Clancy children", Text: "Top English Wikipedia page by pageviews (rank 1).", PublishedAt: "2026-09-02T12:00:00Z"},
		{ID: "w2", Title: "Toxic 2026 film", Text: "Top English Wikipedia page by pageviews (rank 2).", PublishedAt: "2026-09-02T12:00:00Z"},
		{ID: "w3", Title: "Dolly Parton", Text: "Top English Wikipedia page by pageviews (rank 3).", PublishedAt: "2026-09-02T12:00:00Z"},
	}
	clusters := engine.ClusterSignalsV2(values, 0.42)
	if len(clusters) != 3 {
		t.Fatalf("Wikipedia boilerplate over-merged: %+v", clusters)
	}
}

func TestClusterSignalsV2DoesNotTreatGenericListPrefixAsNamedEntity(t *testing.T) {
	values := []model.Signal{
		{
			ID:               "wikipedia:List_of_highest-grossing_films",
			Source:           model.Source{Name: "Wikipedia", Domain: "wikipedia.org"},
			DiscoveryChannel: "wikipedia",
			Title:            "List of highest-grossing films",
			ClusterText:      "List of highest-grossing films",
			PublishedAt:      "2026-09-05T23:00:51Z",
		},
		{
			ID:               "wikipedia:List_of_Intel_codenames",
			Source:           model.Source{Name: "Wikipedia", Domain: "wikipedia.org"},
			DiscoveryChannel: "wikipedia",
			Title:            "List of Intel codenames",
			ClusterText:      "List of Intel codenames",
			PublishedAt:      "2026-09-05T23:00:51Z",
		},
	}
	if engine.SameEvent(values[0], values[1], 0.42) {
		t.Fatal("the generic capitalized word List must not make unrelated Wikipedia pages the same event")
	}
	clusters := engine.ClusterSignalsV2(values, 0.42)
	if len(clusters) != 2 {
		t.Fatalf("production Wikipedia list pages merged: %+v", clusters)
	}
}

func TestClusterSignalsV2RejectsTransitiveBridgeMerges(t *testing.T) {
	values := []model.Signal{
		{ID: "a", Title: "OpenAI launches Atlas browser agent", PublishedAt: "2026-09-03T10:00:00Z"},
		{ID: "b", Title: "OpenAI Atlas browser agent gets Firefox plugin support", PublishedAt: "2026-09-03T10:01:00Z"},
		{ID: "c", Title: "Firefox browser plugin update released", PublishedAt: "2026-09-03T10:02:00Z"},
	}
	if !engine.SameEvent(values[0], values[1], 0.42) || !engine.SameEvent(values[1], values[2], 0.42) {
		t.Fatal("fixture must contain a lexical bridge")
	}
	if engine.SameEvent(values[0], values[2], 0.42) {
		t.Fatal("bridge endpoints should be unrelated")
	}
	clusters := engine.ClusterSignalsV2(values, 0.42)
	if len(clusters) != 2 {
		t.Fatalf("transitive bridge collapsed unrelated endpoints: %+v", clusters)
	}
}
