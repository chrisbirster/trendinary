package scanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

const (
	maxConcurrentDiscoverySources = 12
	perSourceDiscoveryTimeout      = 8 * time.Second
)

type discoveryResult struct {
	index  int
	name   string
	values []model.Signal
	err    error
}

// discoverExtraSources polls independent adapters concurrently while preserving
// deterministic source order in the combined discovery slice. A slow publisher
// gets its own timeout instead of consuming the scanner's entire run budget.
// ScheduledSource still owns cadence/cache behavior, so sources that are not due
// return their cached batch immediately without making a network request.
func discoverExtraSources(ctx context.Context, sources []DiscoverySource) ([]model.Signal, []string) {
	if len(sources) == 0 {
		return nil, nil
	}

	results := make([]discoveryResult, len(sources))
	semaphore := make(chan struct{}, maxConcurrentDiscoverySources)
	var wg sync.WaitGroup
	for index, source := range sources {
		if source == nil {
			continue
		}
		wg.Add(1)
		go func(index int, source DiscoverySource) {
			defer wg.Done()
			result := discoveryResult{index: index, name: source.Name()}
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				result.err = ctx.Err()
				results[index] = result
				return
			}

			sourceCtx, cancel := context.WithTimeout(ctx, perSourceDiscoveryTimeout)
			defer cancel()
			result.values, result.err = source.Discover(sourceCtx)
			results[index] = result
		}(index, source)
	}
	wg.Wait()

	values := make([]model.Signal, 0)
	warnings := make([]string, 0)
	for _, result := range results {
		if len(result.values) > 0 {
			values = append(values, result.values...)
		}
		if result.err != nil {
			name := result.name
			if name == "" {
				name = fmt.Sprintf("source-%d", result.index+1)
			}
			warnings = append(warnings, fmt.Sprintf("%s discovery: %v", name, result.err))
		}
	}
	return values, warnings
}
