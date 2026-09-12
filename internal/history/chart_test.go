package history_test

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
)

func TestApplyChartTracksMovementAndReentry(t *testing.T) {
	store, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	base := time.Date(2026, 9, 12, 20, 0, 0, 0, time.UTC)

	first, err := store.ApplyChart(ctx, []model.Trend{
		{ID: "alpha", Slug: "alpha", Rank: 1, Score: 90, ConfidenceTier: "CONFIRMED"},
		{ID: "beta", Slug: "beta", Rank: 2, Score: 80, ConfidenceTier: "CORROBORATED"},
	}, base)
	if err != nil {
		t.Fatal(err)
	}
	if first[0].Chart.Movement != "NEW" || first[0].Chart.PeakRank != 1 || first[0].Chart.TotalScans != 1 {
		t.Fatalf("unexpected first chart stats: %+v", first[0].Chart)
	}

	second, err := store.ApplyChart(ctx, []model.Trend{
		{ID: "beta", Slug: "beta", Rank: 1, Score: 91, ConfidenceTier: "CONFIRMED"},
	}, base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Chart.Movement != "▲ 1" || second[0].Chart.PreviousRank != 2 || second[0].Chart.PeakRank != 1 {
		t.Fatalf("unexpected movement stats: %+v", second[0].Chart)
	}

	third, err := store.ApplyChart(ctx, []model.Trend{
		{ID: "alpha", Slug: "alpha", Rank: 2, Score: 75, ConfidenceTier: "CORROBORATED"},
		{ID: "beta", Slug: "beta", Rank: 1, Score: 92, ConfidenceTier: "CONFIRMED"},
	}, base.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if third[0].Chart.Movement != "RE" || third[0].Chart.Status != "RE" || third[0].Chart.PeakRank != 1 || third[0].Chart.TotalScans != 2 {
		t.Fatalf("unexpected reentry stats: %+v", third[0].Chart)
	}
	if third[1].Chart.ConsecutiveScans != 3 || third[1].Chart.NumberOneScans != 2 {
		t.Fatalf("unexpected consecutive stats: %+v", third[1].Chart)
	}
}
