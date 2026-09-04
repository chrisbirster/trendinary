package history_test

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
)

func TestTrendSignalsReturnsRecentPersistedEvidence(t *testing.T) {
	ctx := context.Background()
	store, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 9, 3, 15, 0, 0, 0, time.UTC)
	values := []model.Signal{
		{ID: "wired:1", Source: model.Source{Name: "WIRED", Domain: "wired.com"}, DiscoveryChannel: "rss", Title: "Audacity 4.0 released", URL: "https://wired.com/audacity", PublishedAt: now.Add(-time.Hour).Format(time.RFC3339)},
		{ID: "ars:1", Source: model.Source{Name: "Ars Technica", Domain: "arstechnica.com"}, DiscoveryChannel: "gdelt", Title: "Audacity launches version 4", URL: "https://arstechnica.com/audacity", PublishedAt: now.Add(-30 * time.Minute).Format(time.RFC3339)},
	}
	if err := store.RecordSignals(ctx, values); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordTrendSignals(ctx, "trend-audacity", values, now); err != nil {
		t.Fatal(err)
	}

	got, err := store.TrendSignals(ctx, "trend-audacity", now.Add(-24*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("evidence = %+v", got)
	}
	if got[0].Source.Domain == "" || got[1].Source.Domain == "" {
		t.Fatalf("publisher domains were not restored: %+v", got)
	}
	channels := map[string]bool{}
	for _, signal := range got {
		channels[signal.DiscoveryChannel] = true
	}
	if !channels["rss"] || !channels["gdelt"] {
		t.Fatalf("discovery channels were not restored: %+v", got)
	}
}

func TestTrendMembershipRefreshPreservesFirstObservation(t *testing.T) {
	ctx := context.Background()
	store, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	first := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	last := first.Add(90 * time.Minute)
	signal := model.Signal{ID: "wired:audacity", Source: model.Source{Name: "WIRED", Domain: "wired.com"}, DiscoveryChannel: "rss", Title: "Audacity 4.0"}
	if err := store.RecordSignals(ctx, []model.Signal{signal}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordTrendSignals(ctx, "trend-audacity", []model.Signal{signal}, first); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordTrendSignals(ctx, "trend-audacity", []model.Signal{signal}, last); err != nil {
		t.Fatal(err)
	}

	var firstRaw, lastRaw string
	if err := store.DB().QueryRowContext(ctx, `
SELECT first_observed_at, observed_at
FROM trend_signal_memberships
WHERE trend_key = ? AND signal_id = ?`, "trend-audacity", signal.ID).Scan(&firstRaw, &lastRaw); err != nil {
		t.Fatal(err)
	}
	firstGot, err := time.Parse(time.RFC3339Nano, firstRaw)
	if err != nil {
		t.Fatal(err)
	}
	lastGot, err := time.Parse(time.RFC3339Nano, lastRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !firstGot.Equal(first) || !lastGot.Equal(last) {
		t.Fatalf("membership first=%s last=%s, want %s / %s", firstGot, lastGot, first, last)
	}
}
