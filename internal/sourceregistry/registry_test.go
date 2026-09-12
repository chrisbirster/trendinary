package sourceregistry_test

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/sourceregistry"
)

func TestDefaultRegistryHasBroadEnabledRSSCoverage(t *testing.T) {
	feeds := sourceregistry.EnabledRSS()
	if len(feeds) < 30 {
		t.Fatalf("enabled RSS feeds = %d, want at least 30", len(feeds))
	}
	for _, feed := range feeds {
		if feed.URL == "" || feed.Cadence <= 0 || feed.Policy != sourceregistry.PolicyOfficialRSS {
			t.Fatalf("invalid enabled feed: %+v", feed)
		}
	}
}

func TestGoogleTrendsTrendingNowIsEnabled(t *testing.T) {
	for _, entry := range sourceregistry.Default() {
		if entry.ID != "google-trends-us" {
			continue
		}
		if !entry.Enabled || entry.Kind != "rss" || entry.Policy != sourceregistry.PolicyOfficialRSS {
			t.Fatalf("Google Trends RSS entry must be enabled and official: %+v", entry)
		}
		if entry.URL != "https://trends.google.com/trending/rss?geo=US" {
			t.Fatalf("unexpected Google Trends URL: %s", entry.URL)
		}
		return
	}
	t.Fatal("Google Trends registry entry missing")
}

func TestGoogleTrendsAPIAlphaRemainsOptional(t *testing.T) {
	for _, entry := range sourceregistry.Default() {
		if entry.ID != "google-trends-api" {
			continue
		}
		if entry.Enabled || entry.Policy != sourceregistry.PolicyOfficialAPI {
			t.Fatalf("Google Trends API alpha must stay optional until access is configured: %+v", entry)
		}
		return
	}
	t.Fatal("Google Trends API registry entry missing")
}

func TestAPRemainsLicenseGated(t *testing.T) {
	for _, entry := range sourceregistry.Default() {
		if entry.ID != "ap-media-api" {
			continue
		}
		if entry.Enabled || entry.Policy != sourceregistry.PolicyLicenseRequired {
			t.Fatalf("AP entry must stay license gated: %+v", entry)
		}
		return
	}
	t.Fatal("AP registry entry missing")
}
