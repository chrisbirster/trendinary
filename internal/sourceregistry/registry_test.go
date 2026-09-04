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
	publishers := map[string]bool{}
	for _, feed := range feeds {
		if feed.URL == "" || feed.Cadence <= 0 || feed.Policy != sourceregistry.PolicyOfficialRSS {
			t.Fatalf("invalid enabled feed: %+v", feed)
		}
		publishers[feed.Name] = true
	}
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
