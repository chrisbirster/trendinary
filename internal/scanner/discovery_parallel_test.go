package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type parallelTestSource struct {
	name    string
	started chan<- string
	release <-chan struct{}
}

func (s *parallelTestSource) Name() string { return s.name }

func (s *parallelTestSource) Discover(ctx context.Context) ([]model.Signal, error) {
	s.started <- s.name
	select {
	case <-s.release:
		return []model.Signal{{ID: s.name, Title: s.name}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestDiscoverExtraSourcesRunsConcurrentlyAndPreservesOrder(t *testing.T) {
	started := make(chan string, 4)
	release := make(chan struct{})
	sources := []DiscoverySource{
		&parallelTestSource{name: "one", started: started, release: release},
		&parallelTestSource{name: "two", started: started, release: release},
		&parallelTestSource{name: "three", started: started, release: release},
		&parallelTestSource{name: "four", started: started, release: release},
	}

	type outcome struct {
		values   []model.Signal
		warnings []string
	}
	done := make(chan outcome, 1)
	go func() {
		values, warnings := discoverExtraSources(context.Background(), sources)
		done <- outcome{values: values, warnings: warnings}
	}()

	for i := 0; i < len(sources); i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("discovery sources did not start concurrently")
		}
	}
	close(release)

	select {
	case result := <-done:
		if len(result.warnings) != 0 {
			t.Fatalf("warnings = %v", result.warnings)
		}
		if len(result.values) != 4 {
			t.Fatalf("values = %d, want 4", len(result.values))
		}
		for i, want := range []string{"one", "two", "three", "four"} {
			if result.values[i].ID != want {
				t.Fatalf("value[%d] = %q, want %q", i, result.values[i].ID, want)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("discovery did not finish after release")
	}
}
