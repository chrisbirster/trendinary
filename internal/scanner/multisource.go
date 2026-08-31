package scanner

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/signals"
)

// The v0.1 lexical clusterer performs pairwise similarity comparisons. Keep
// the ingestion window large for durability/replay context, but bound each
// scoring pass until the clustering implementation moves to an indexed or
// semantic candidate-generation strategy.
const maxStreamingDiscoverySignals = 1500

// RunWithRecent treats the bounded streaming window as a first-class discovery
// source alongside Hacker News. Either source may be temporarily unavailable;
// a scan only fails when no discovery signals remain at all.
func (s *Scanner) RunWithRecent(ctx context.Context, live *recent.Store) (Result, error) {
	if s.history == nil || s.memory == nil {
		return Result{}, fmt.Errorf("scanner dependencies are incomplete")
	}

	warnings := make([]string, 0)
	discovery := make([]model.Signal, 0)
	var hnErr error
	if s.hn != nil {
		hnItems, err := s.hn.Top(ctx, s.config.HackerNewsLimit)
		if err != nil {
			hnErr = err
			warnings = append(warnings, fmt.Sprintf("hacker news discovery: %v", err))
		} else {
			discovery = append(discovery, signals.HackerNews(hnItems)...)
		}
	}
	if live != nil {
		streamSignals := live.Recent(time.Time{})
		if len(streamSignals) > maxStreamingDiscoverySignals {
			streamSignals = streamSignals[len(streamSignals)-maxStreamingDiscoverySignals:]
		}
		discovery = append(discovery, streamSignals...)
	}
	discovery = deduplicateSignals(discovery)
	if len(discovery) == 0 {
		if hnErr != nil {
			return Result{}, fmt.Errorf("no discovery signals available: %w", hnErr)
		}
		return Result{}, fmt.Errorf("no discovery signals available")
	}

	seedClusters := engine.ClusterSignals(discovery, s.config.ClusterThreshold)
	sort.SliceStable(seedClusters, func(i, j int) bool {
		return clusterEngagement(seedClusters[i]) > clusterEngagement(seedClusters[j])
	})

	enriched := make([]engine.Cluster, len(seedClusters))
	copy(enriched, seedClusters)
	if s.bluesky != nil {
		limit := s.config.EnrichClusters
		if limit > len(enriched) {
			limit = len(enriched)
		}
		for index := 0; index < limit; index++ {
			query := clusterQuery(enriched[index])
			if query == "" {
				continue
			}
			response, searchErr := s.bluesky.Search(ctx, query, s.config.BlueskyLimit)
			if searchErr != nil {
				warnings = append(warnings, fmt.Sprintf("bluesky %q: %v", query, searchErr))
				continue
			}
			enriched[index].Signals = deduplicateSignals(append(enriched[index].Signals, signals.Bluesky(response.Posts)...))
		}
	}

	allSignals := make([]model.Signal, 0)
	for index := range enriched {
		enriched[index].Signals = deduplicateSignals(enriched[index].Signals)
		allSignals = append(allSignals, enriched[index].Signals...)
	}
	allSignals = deduplicateSignals(allSignals)
	if err := s.history.RecordSignals(ctx, allSignals); err != nil {
		return Result{}, fmt.Errorf("persist signals: %w", err)
	}

	now := s.now().UTC()
	trends := make([]model.Trend, 0, len(enriched))
	for _, cluster := range enriched {
		trend, snapshot, scoreErr := s.scoreCluster(ctx, cluster, now)
		if scoreErr != nil {
			warnings = append(warnings, fmt.Sprintf("score %s: %v", cluster.Key, scoreErr))
			continue
		}
		if err := s.history.RecordSnapshot(ctx, snapshot); err != nil {
			warnings = append(warnings, fmt.Sprintf("snapshot %s: %v", cluster.Key, err))
			continue
		}
		trends = append(trends, trend)
	}

	sort.SliceStable(trends, func(i, j int) bool {
		if trends[i].Score == trends[j].Score {
			return trends[i].Slug < trends[j].Slug
		}
		return trends[i].Score > trends[j].Score
	})
	if len(trends) > s.config.PublishedTrendLimit {
		trends = trends[:s.config.PublishedTrendLimit]
	}
	for index := range trends {
		trends[index].Rank = index + 1
	}
	s.memory.ReplaceTrends(trends)

	return Result{
		Signals:  len(allSignals),
		Clusters: len(enriched),
		Trends:   len(trends),
		Warnings: warnings,
	}, nil
}
