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
