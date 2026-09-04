package editorial

import (
	"context"
	"testing"
	"time"
)

func TestSourceAnalyticsSeparatesPublisherAndDiscoveryChannel(t *testing.T) {
	store := openQualityTestStore(t)
	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx, `
CREATE TABLE signals (
 id TEXT PRIMARY KEY,
 source_name TEXT NOT NULL,
 source_domain TEXT,
 discovery_channel TEXT NOT NULL DEFAULT '',
 observed_at TEXT NOT NULL
);
CREATE TABLE trend_signal_memberships (
 trend_key TEXT NOT NULL,
 signal_id TEXT NOT NULL,
 first_observed_at TEXT NOT NULL DEFAULT '',
 observed_at TEXT NOT NULL,
 PRIMARY KEY (trend_key, signal_id)
);
CREATE TABLE trend_snapshots (
 trend_key TEXT NOT NULL,
 observed_at TEXT NOT NULL,
 lifecycle TEXT NOT NULL
);`); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	values := []struct {
		id, name, domain, channel, trend string
		firstAt                         time.Time
		lastAt                          time.Time
	}{
		{"wired-1", "WIRED", "wired.com", "hacker-news", "audacity", base, base.Add(45 * time.Minute)},
		{"ars-1", "Ars Technica", "arstechnica.com", "rss", "audacity", base.Add(10 * time.Minute), base.Add(40 * time.Minute)},
		{"wired-2", "WIRED", "wired.com", "rss", "other", base.Add(20 * time.Minute), base.Add(20 * time.Minute)},
	}
	for _, value := range values {
		lastAt := value.lastAt.Format(time.RFC3339Nano)
		firstAt := value.firstAt.Format(time.RFC3339Nano)
		if _, err := store.db.ExecContext(ctx, `INSERT INTO signals (id, source_name, source_domain, discovery_channel, observed_at) VALUES (?, ?, ?, ?, ?)`, value.id, value.name, value.domain, value.channel, lastAt); err != nil {
			t.Fatal(err)
		}
		if _, err := store.db.ExecContext(ctx, `INSERT INTO trend_signal_memberships (trend_key, signal_id, first_observed_at, observed_at) VALUES (?, ?, ?, ?)`, value.trend, value.id, firstAt, lastAt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO trend_snapshots (trend_key, observed_at, lifecycle) VALUES (?, ?, 'BREAKING')`, "audacity", base.Add(time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}

	report, err := store.SourceAnalytics(ctx, base.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Publishers) < 2 || report.Publishers[0].Key != "wired.com" {
		t.Fatalf("publishers = %+v", report.Publishers)
	}
	wired := report.Publishers[0]
	if wired.Signals != 2 || wired.Trends != 2 || wired.FirstHits != 2 || wired.SoloTrends != 1 {
		t.Fatalf("wired contribution = %+v", wired)
	}
	var hn SourceContribution
	for _, value := range report.Channels {
		if value.Key == "hacker-news" {
			hn = value
		}
	}
	if hn.Signals != 1 || hn.Trends != 1 || hn.FirstHits != 1 {
		t.Fatalf("HN contribution = %+v", hn)
	}
	// wired-1 was refreshed at +45m, but its immutable first membership is the
	// origin. Lead time must therefore stay 60m rather than collapsing to 15m.
	if wired.AverageLeadToBreakingMinutes != 60 {
		t.Fatalf("wired lead = %f, want 60", wired.AverageLeadToBreakingMinutes)
	}
}
