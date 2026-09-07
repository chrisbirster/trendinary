package store_test

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/store"
)

func TestNewMemoryStartsWithoutPrototypeTrends(t *testing.T) {
	if trends := store.NewMemory().Trends(); len(trends) != 0 {
		t.Fatalf("production memory started with %d prototype trends: %+v", len(trends), trends)
	}
}

func TestNewDemoMemoryRetainsExplicitFixtures(t *testing.T) {
	trends := store.NewDemoMemory().Trends()
	if len(trends) == 0 || trends[0].Slug != "at-protocol" {
		t.Fatalf("demo fixture missing expected seed trend: %+v", trends)
	}
}
