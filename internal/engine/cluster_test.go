package engine_test

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

func TestClusterSignals(t *testing.T) {
	signals := []model.Signal{
		{ID: "1", Title: "AT Protocol social apps are accelerating"},
		{ID: "2", Text: "New social apps built on AT Protocol are launching"},
		{ID: "3", Title: "Orbit Cup ends with last second goal"},
	}

	clusters := engine.ClusterSignals(signals, 0.35)
	if len(clusters) != 2 {
		t.Fatalf("clusters = %d, want 2: %+v", len(clusters), clusters)
	}
	if len(clusters[0].Signals) != 2 {
		t.Fatalf("largest cluster = %d signals, want 2", len(clusters[0].Signals))
	}
}

func TestClusterSignalsIgnoresDescriptiveBoilerplateWhenTitleExists(t *testing.T) {
	values := []model.Signal{
		{ID: "wiki:1", Title: "Killing of the Clancy children", Text: "Top English Wikipedia page by pageviews (rank 1)."},
		{ID: "wiki:2", Title: "Toxic 2026 film", Text: "Top English Wikipedia page by pageviews (rank 2)."},
		{ID: "wiki:3", Title: "Dolly Parton", Text: "Top English Wikipedia page by pageviews (rank 3)."},
	}
	clusters := engine.ClusterSignals(values, 0.42)
	if len(clusters) != 3 {
		t.Fatalf("clusters = %d, want 3: %+v", len(clusters), clusters)
	}
}

func TestSignalTermsDoNotIncludePublisherDomain(t *testing.T) {
	left := model.Signal{Title: "Alpha chip launch", URL: "https://wired.com/story/alpha-chip"}
	right := model.Signal{Title: "Completely unrelated gardening guide", URL: "https://wired.com/story/garden"}
	clusters := engine.ClusterSignals([]model.Signal{left, right}, 0.20)
	if len(clusters) != 2 {
		t.Fatalf("same publisher must not merge unrelated events: %+v", clusters)
	}
}

func TestClusterTextOverridesDescriptiveText(t *testing.T) {
	signal := model.Signal{Title: "Visible title", Text: "shared boilerplate", ClusterText: "specific event phrase"}
	if got := engine.ClusteringText(signal); got != "specific event phrase" {
		t.Fatalf("ClusteringText = %q", got)
	}
}
