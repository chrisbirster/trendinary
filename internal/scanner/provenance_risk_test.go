package scanner

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/model"
)

func TestScoringProvenanceDiscountsGitHubOnlyPublisherBurst(t *testing.T) {
	value := model.TrendProvenance{
		PublisherCount: 8,
		PlatformCount:  1,
		Publishers:     []string{"a", "b", "c", "d", "e", "f", "g", "h"},
		Platforms:      []string{"GitHub"},
	}
	got := scoringProvenance(value)
	if got.PublisherCount != 1 {
		t.Fatalf("publisher count = %d, want 1", got.PublisherCount)
	}
	if len(got.Publishers) != 8 {
		t.Fatalf("display publishers were modified: %v", got.Publishers)
	}
}

func TestScoringProvenanceKeepsCrossPlatformBreadth(t *testing.T) {
	value := model.TrendProvenance{
		PublisherCount: 3,
		PlatformCount:  2,
		Publishers:     []string{"repo-owner", "WIRED", "TechCrunch"},
		Platforms:      []string{"GitHub", "Web"},
	}
	got := scoringProvenance(value)
	if got.PublisherCount != 3 {
		t.Fatalf("publisher count = %d, want 3", got.PublisherCount)
	}
}
