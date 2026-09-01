package propagation_test

import (
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/propagation"
)

func TestBuildOrdersSourcesByFirstSeenAndAggregates(t *testing.T) {
	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	signals := []model.Signal{
		{ID: "b1", Source: model.Source{Name: "Bluesky", Domain: "bsky.app"}, PublishedAt: base.Add(20 * time.Minute).Format(time.RFC3339), Engagement: model.Engagement{Likes: 10}},
		{ID: "g1", Source: model.Source{Name: "GitHub", Domain: "github.com"}, PublishedAt: base.Format(time.RFC3339), Engagement: model.Engagement{Score: 30, Reposts: 2}},
		{ID: "b2", Source: model.Source{Name: "Bluesky", Domain: "bsky.app"}, PublishedAt: base.Add(25 * time.Minute).Format(time.RFC3339), Engagement: model.Engagement{Likes: 15}},
	}

	path := propagation.Build(signals, base)
	if len(path) != 2 { t.Fatalf("path len = %d, want 2", len(path)) }
	if path[0].Source.Domain != "github.com" || path[1].Source.Domain != "bsky.app" {
		t.Fatalf("unexpected propagation order: %+v", path)
	}
	if path[1].SignalCount != 2 || path[1].Engagement != 25 {
		t.Fatalf("bluesky aggregate = %+v", path[1])
	}
}
