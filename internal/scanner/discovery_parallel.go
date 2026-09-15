package scanner

import (
	"context"
	"fmt"
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

type discoveryTimeoutProvider interface {
	DiscoveryTimeout() time.Duration
}

func discoveryTimeout(source DiscoverySource) time.Duration {
	if provider, ok := source.(discoveryTimeoutProvider); ok {
		if timeout := provider.DiscoveryTimeout(); timeout > 0 {
			return timeout
		}
	}
	return perSourceDiscoveryTimeout
}

// discoverExtraSources polls independent adapters concurrently while preserving
// deterministic source order in the combined discovery slice. A slow publisher
// gets its own timeout instead of consuming the scanner's entire run budget.
// ScheduledSource still owns cadence/cache behavior, so sources that are not due
// return their cached batch immediately without making a network request.
//
// The parent scanner deadline is also authoritative. We must not wait forever
// for an adapter that ignores context cancellation; otherwise a single bad
// source can leave the scanner permanently marked running.
func discoverExtraSources(ctx context.Context, sources []DiscoverySource) ([]model.Signal, []string) {
	if len(sources) == 0 {
		return nil, nil
	}

	results := make([]discoveryResult, len(sources))
	completed := make([]bool, len(sources))
	semaphore := make(chan struct{}, maxConcurrentDiscoverySources)
	resultCh := make(chan discoveryResult, len(sources))
	pending := 0

	for index, source := range sources {
		if source == nil {
			continue
		}
		pending++
		go func(index int, source DiscoverySource) {
			result := discoveryResult{index: index, name: source.Name()}
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				result.err = ctx.Err()
				resultCh <- result
				return
			}

			sourceCtx, cancel := context.WithTimeout(ctx, discoveryTimeout(source))
			defer cancel()
			result.values, result.err = source.Discover(sourceCtx)
			// resultCh is sized for every source, so a misbehaving adapter that
			// returns after the parent deadline cannot block forever on send.
			resultCh <- result
		}(index, source)
	}

	received := 0
	for received < pending {
		select {
		case result := <-resultCh:
			results[result.index] = result
			completed[result.index] = true
			received++
		case <-ctx.Done():
			// Preserve any results already waiting in the buffered channel before
			// returning partial discovery data at the scanner deadline.
			for {
				select {
				case result := <-resultCh:
					if !completed[result.index] {
						results[result.index] = result
						completed[result.index] = true
						received++
					}
				default:
					values, warnings := collectDiscoveryResults(results, completed)
					warnings = append(warnings, fmt.Sprintf("discovery deadline: %v (%d source(s) unfinished)", ctx.Err(), pending-received))
					return values, warnings
				}
			}
		}
	}

	return collectDiscoveryResults(results, completed)
}

func collectDiscoveryResults(results []discoveryResult, completed []bool) ([]model.Signal, []string) {
	values := make([]model.Signal, 0)
	warnings := make([]string, 0)
	for index, result := range results {
		if index >= len(completed) || !completed[index] {
			continue
		}
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
