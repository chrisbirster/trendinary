package scanner

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/signals"
)

const rollingEvidenceWindow = 24 * time.Hour

// RunWithSourcesV2 is the calibrated multi-source path. Detection Quality v3
// builds on this pipeline by persisting cluster memberships for human replay
// evaluation and scoring the resulting stable entities with Trendinary Score v4.
func (s *Scanner) RunWithSourcesV2(ctx context.Context, live *recent.Store, extra []DiscoverySource) (Result, error) {
	if s.history == nil || s.memory == nil {
		return Result{}, fmt.Errorf("scanner dependencies are incomplete")
	}
	warnings := []string{}
	discovery := []model.Signal{}
	var hnErr error
	if s.hn != nil {
		items, err := s.hn.Top(ctx, s.config.HackerNewsLimit)
		if err != nil {
			hnErr = err
			warnings = append(warnings, fmt.Sprintf("hacker news discovery: %v", err))
		} else {
			discovery = append(discovery, signals.HackerNews(items)...)
		}
	}
	if live != nil {
		stream := live.Recent(time.Time{})
		if len(stream) > maxStreamingDiscoverySignals {
			stream = stream[len(stream)-maxStreamingDiscoverySignals:]
		}
		discovery = append(discovery, stream...)
	}
	extraSignals, extraWarnings := discoverExtraSources(ctx, extra)
	discovery = append(discovery, extraSignals...)
	warnings = append(warnings, extraWarnings...)
	discovery = deduplicateSignals(discovery)
	filtered := filterDiscoveryNoise(discovery)
	discovery = filtered.Signals
	if filtered.Suppressed > 0 {
		warnings = append(warnings, fmt.Sprintf("noise controls suppressed %d repeated/flood/low-information signals", filtered.Suppressed))
	}
	if len(discovery) == 0 {
		if hnErr != nil {
			return Result{}, fmt.Errorf("no discovery signals available: %w", hnErr)
		}
		return Result{}, fmt.Errorf("no discovery signals available")
	}

	clustered := engine.ClusterSignalsV2(discovery, s.config.ClusterThreshold)
	candidates := make([]engine.Cluster, 0, len(clustered))
	for _, cluster := range clustered {
		if chartCandidateCluster(cluster) {
			candidates = append(candidates, cluster)
		}
	}
	seedClusters := strongestClusters(candidates, topCandidateProcessingLimit(s.config.PublishedTrendLimit))
	if len(seedClusters) == 0 {
		return Result{Signals: len(discovery), Warnings: append(warnings, "no clusters met the Top 20 chart candidate gate")}, nil
	}

	enriched := append([]engine.Cluster(nil), seedClusters...)
	hydrationLimit := s.config.EnrichClusters
	if hydrationLimit > len(enriched) {
		hydrationLimit = len(enriched)
	}
	for i := 0; i < hydrationLimit; i++ {
		if err := s.hydrateBlueskyCandidate(ctx, &enriched[i]); err != nil {
			warnings = append(warnings, fmt.Sprintf("bluesky hydrate %q: %v", enriched[i].Key, err))
		}
	}
	if s.bluesky != nil {
		limit := s.config.EnrichClusters
		if limit > len(enriched) {
			limit = len(enriched)
		}
		for i := 0; i < limit; i++ {
			query := clusterQuery(enriched[i])
			if query == "" {
				continue
			}
			response, err := s.bluesky.Search(ctx, query, s.config.BlueskyLimit)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("bluesky %q: %v", query, err))
				continue
			}
			enriched[i].Signals = deduplicateSignals(append(enriched[i].Signals, signals.Bluesky(response.Posts)...))
			s.enrichBlueskyProfiles(ctx, &enriched[i])
		}
	}

	allSignals := []model.Signal{}
	for i := range enriched {
		enriched[i].Signals = deduplicateSignals(enriched[i].Signals)
		allSignals = append(allSignals, enriched[i].Signals...)
	}
	allSignals = deduplicateSignals(allSignals)

	type scoredCandidate struct {
		trend    model.Trend
		snapshot history.Snapshot
		entity   history.Entity
		current  engine.Cluster
		evidence engine.Cluster
	}

	now := s.now().UTC()
	calibration, calibrationErr := s.history.Calibration(ctx, now.Add(-30*24*time.Hour))
	if calibrationErr != nil {
		warnings = append(warnings, fmt.Sprintf("score calibration: %v", calibrationErr))
	}
	scored := make([]scoredCandidate, 0, len(enriched))
	for _, cluster := range enriched {
		entity, err := s.history.ResolveEntityStrict(ctx, clusterName(cluster), cluster.Key, engine.CanonicalTerms(cluster.Signals, 8), now)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("identity %s: %v", cluster.Key, err))
			continue
		}

		historical, err := s.history.TrendSignals(ctx, entity.ID, now.Add(-rollingEvidenceWindow), 1000)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("rolling evidence %s: %v", entity.ID, err))
			historical = nil
		}
		related := relatedEvidence(cluster.Signals, historical, s.config.ClusterThreshold)
		evidenceSignals := deduplicateSignals(append(append([]model.Signal(nil), cluster.Signals...), related...))
		current := cluster
		current.Key = entity.ID
		evidence := engine.Cluster{Key: entity.ID, Signals: evidenceSignals}

		trend, snapshot, err := s.scoreClusterV3(ctx, current, evidence, now, calibration)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("score %s: %v", entity.ID, err))
			continue
		}
		trend.ID = entity.ID
		trend.Slug = entity.Slug
		trend.Aliases = entity.Aliases
		trend.Name = clusterName(cluster)
		scored = append(scored, scoredCandidate{
			trend: trend, snapshot: snapshot, entity: entity, current: current, evidence: evidence,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].trend.Score == scored[j].trend.Score {
			return scored[i].trend.Slug < scored[j].trend.Slug
		}
		return scored[i].trend.Score > scored[j].trend.Score
	})
	if len(scored) > s.config.PublishedTrendLimit {
		scored = scored[:s.config.PublishedTrendLimit]
	}

	selectedSignals := make([]model.Signal, 0)
	for _, candidate := range scored {
		selectedSignals = append(selectedSignals, candidate.current.Signals...)
	}
	selectedSignals = deduplicateSignals(selectedSignals)
	if err := s.history.RecordSignalsBatch(ctx, selectedSignals); err != nil {
		return Result{}, fmt.Errorf("persist selected signals: %w", err)
	}

	trends := make([]model.Trend, 0, len(scored))
	for _, candidate := range scored {
		if err := s.history.RecordTrendSignalsBatch(ctx, candidate.entity.ID, candidate.current.Signals, now); err != nil {
			warnings = append(warnings, fmt.Sprintf("quality membership %s: %v", candidate.entity.ID, err))
		}
		trend := candidate.trend
		if err := s.decorateTrend(ctx, &trend, candidate.entity, candidate.evidence, now); err != nil {
			warnings = append(warnings, fmt.Sprintf("decorate %s: %v", candidate.entity.ID, err))
		}
		if err := s.history.RecordSnapshot(ctx, candidate.snapshot); err != nil {
			warnings = append(warnings, fmt.Sprintf("snapshot %s: %v", candidate.entity.ID, err))
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
	for i := range trends {
		trends[i].Rank = i + 1
	}
	charted, err := s.history.ApplyChart(ctx, trends, now)
	if err != nil {
		return Result{}, fmt.Errorf("persist chart: %w", err)
	}
	trends = charted
	s.memory.ReplaceTrends(trends)
	return Result{Signals: len(allSignals), Clusters: len(enriched), Trends: len(trends), Warnings: warnings}, nil
}
