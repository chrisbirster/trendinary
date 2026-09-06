package scanner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/signals"
)

const (
	maxStreamingDiscoverySignals      = 1500
	maxCandidateClusters               = 100
	minIndependentCandidateSignals     = 2
	minIndependentCommunityActors      = 2
)

type DiscoverySource interface {
	Name() string
	Discover(context.Context) ([]model.Signal, error)
}

type blueskyHydrator interface {
	HydratePosts(context.Context, []string) (map[string]bluesky.Post, error)
}

type blueskyProfiler interface {
	Profiles(context.Context, []string) map[string]bluesky.Author
}

func (s *Scanner) RunWithRecent(ctx context.Context, live *recent.Store) (Result, error) {
	return s.RunWithSources(ctx, live, nil)
}

func (s *Scanner) RunWithSources(ctx context.Context, live *recent.Store, extra []DiscoverySource) (Result, error) {
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
	for _, source := range extra {
		if source == nil {
			continue
		}
		values, err := source.Discover(ctx)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s discovery: %v", source.Name(), err))
			continue
		}
		discovery = append(discovery, values...)
	}
	discovery = deduplicateSignals(discovery)
	if len(discovery) == 0 {
		if hnErr != nil {
			return Result{}, fmt.Errorf("no discovery signals available: %w", hnErr)
		}
		return Result{}, fmt.Errorf("no discovery signals available")
	}

	clustered := engine.ClusterSignals(discovery, s.config.ClusterThreshold)
	seedClusters := make([]engine.Cluster, 0, len(clustered))
	for _, cluster := range clustered {
		if candidateCluster(cluster) {
			seedClusters = append(seedClusters, cluster)
		}
	}
	sort.SliceStable(seedClusters, func(i, j int) bool {
		left, right := candidateWeight(seedClusters[i]), candidateWeight(seedClusters[j])
		if left == right {
			return seedClusters[i].Key < seedClusters[j].Key
		}
		return left > right
	})
	if len(seedClusters) > maxCandidateClusters {
		seedClusters = seedClusters[:maxCandidateClusters]
	}
	if len(seedClusters) == 0 {
		return Result{Signals: len(discovery), Clusters: 0, Trends: 0, Warnings: append(warnings, "no clusters met the trend candidate gate")}, nil
	}

	enriched := make([]engine.Cluster, len(seedClusters))
	copy(enriched, seedClusters)
	hydrationLimit := s.config.EnrichClusters
	if hydrationLimit > len(enriched) {
		hydrationLimit = len(enriched)
	}
	for index := 0; index < hydrationLimit; index++ {
		if err := s.hydrateBlueskyCandidate(ctx, &enriched[index]); err != nil {
			warnings = append(warnings, fmt.Sprintf("bluesky hydrate %q: %v", enriched[index].Key, err))
		}
	}

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
			s.enrichBlueskyProfiles(ctx, &enriched[index])
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
	for _, currentCluster := range enriched {
		entity, identityErr := s.history.ResolveEntity(ctx, clusterName(currentCluster), currentCluster.Key, engine.CanonicalTerms(currentCluster.Signals, 8), now)
		if identityErr != nil {
			warnings = append(warnings, fmt.Sprintf("identity %s: %v", currentCluster.Key, identityErr))
			continue
		}

		stableCluster := currentCluster
		stableCluster.Key = entity.ID
		trend, snapshot, scoreErr := s.scoreCluster(ctx, stableCluster, now)
		if scoreErr != nil {
			warnings = append(warnings, fmt.Sprintf("score %s: %v", entity.ID, scoreErr))
			continue
		}
		trend.ID = entity.ID
		trend.Slug = entity.Slug
		trend.Aliases = entity.Aliases
		trend.Name = clusterName(currentCluster)
		if err := s.decorateTrend(ctx, &trend, entity, currentCluster, now); err != nil {
			warnings = append(warnings, fmt.Sprintf("decorate %s: %v", entity.ID, err))
		}
		if err := s.history.RecordSnapshot(ctx, snapshot); err != nil {
			warnings = append(warnings, fmt.Sprintf("snapshot %s: %v", entity.ID, err))
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

	return Result{Signals: len(allSignals), Clusters: len(enriched), Trends: len(trends), Warnings: warnings}, nil
}

func (s *Scanner) hydrateBlueskyCandidate(ctx context.Context, cluster *engine.Cluster) error {
	if s.bluesky == nil || cluster == nil {
		return nil
	}
	hydrator, ok := s.bluesky.(blueskyHydrator)
	if !ok {
		return nil
	}
	uris := make([]string, 0)
	for _, signal := range cluster.Signals {
		if signal.Source.Domain != "bsky.app" || !strings.HasPrefix(signal.ID, "bsky:at://") {
			continue
		}
		uris = append(uris, strings.TrimPrefix(signal.ID, "bsky:"))
	}
	if len(uris) == 0 {
		return nil
	}
	posts, err := hydrator.HydratePosts(ctx, uris)
	if err != nil {
		return err
	}
	if len(posts) == 0 {
		return nil
	}
	values := make([]bluesky.Post, 0, len(posts))
	for _, post := range posts {
		values = append(values, post)
	}
	cluster.Signals = deduplicateSignals(append(cluster.Signals, signals.Bluesky(values)...))
	return nil
}

func (s *Scanner) enrichBlueskyProfiles(ctx context.Context, cluster *engine.Cluster) {
	if s.bluesky == nil || cluster == nil {
		return
	}
	profiler, ok := s.bluesky.(blueskyProfiler)
	if !ok {
		return
	}
	actors := make([]string, 0)
	for _, signal := range cluster.Signals {
		if signal.Source.Domain == "bsky.app" && signal.Actor == nil && signal.AuthorID != "" {
			actors = append(actors, signal.AuthorID)
		}
	}
	profiles := profiler.Profiles(ctx, actors)
	for index := range cluster.Signals {
		signal := &cluster.Signals[index]
		profile, ok := profiles[signal.AuthorID]
		if !ok {
			continue
		}
		signal.Author = profile.Handle
		if signal.Author == "" {
			signal.Author = profile.DID
		}
		signal.Actor = &model.ActorProfile{DID: profile.DID, Handle: profile.Handle, DisplayName: profile.DisplayName, Avatar: profile.Avatar}
	}
}

func candidateCluster(cluster engine.Cluster) bool {
	qualifyingSignals := 0
	publishers := make(map[string]struct{})
	communityActors := make(map[string]struct{})

	for _, signal := range cluster.Signals {
		if contextOnlyCandidateSignal(signal) {
			continue
		}
		qualifyingSignals++
		if key := candidateSourceKey(signal); key != "" {
			publishers[key] = struct{}{}
		}
		if key := candidateCommunityActorKey(signal); key != "" {
			communityActors[key] = struct{}{}
		}
	}

	if qualifyingSignals < minIndependentCandidateSignals {
		return false
	}
	return len(publishers) >= 2 || len(communityActors) >= minIndependentCommunityActors
}

func contextOnlyCandidateSignal(signal model.Signal) bool {
	domain := strings.ToLower(strings.TrimSpace(signal.Source.Domain))
	channel := strings.ToLower(strings.TrimSpace(signal.DiscoveryChannel))
	return domain == "wikipedia.org" || channel == "wikipedia"
}

func candidateSourceKey(signal model.Signal) string {
	key := strings.ToLower(strings.TrimSpace(signal.Source.Domain))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(signal.Source.Name))
	}
	return key
}

func candidateCommunityActorKey(signal model.Signal) string {
	domain := strings.ToLower(strings.TrimSpace(signal.Source.Domain))
	channel := strings.ToLower(strings.TrimSpace(signal.DiscoveryChannel))
	community := domain == "bsky.app" || domain == "github.com" || channel == "bluesky" || channel == "hacker-news" || channel == "github"
	if !community {
		return ""
	}
	identity := strings.TrimSpace(signal.AuthorID)
	if identity == "" {
		identity = strings.TrimSpace(signal.Author)
	}
	if identity == "" {
		return ""
	}
	if channel == "" {
		channel = domain
	}
	return channel + ":" + strings.ToLower(identity)
}

func candidateWeight(cluster engine.Cluster) int {
	communityActors := make(map[string]struct{})
	publishers := make(map[string]struct{})
	qualifyingSignals := 0
	engagement := 0

	for _, signal := range cluster.Signals {
		if contextOnlyCandidateSignal(signal) {
			continue
		}
		qualifyingSignals++
		if key := candidateSourceKey(signal); key != "" {
			publishers[key] = struct{}{}
		}
		if key := candidateCommunityActorKey(signal); key != "" {
			communityActors[key] = struct{}{}
		}
		engagement += signal.Engagement.Score + signal.Engagement.Likes + signal.Engagement.Reposts + signal.Engagement.Replies + signal.Engagement.Quotes
	}

	return engagement + qualifyingSignals*10 + len(communityActors)*25 + len(publishers)*50
}
