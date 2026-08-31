package scanner

import (
	"context"
	"sort"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/explain"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/perspective"
	"github.com/chrisbirster/trendinary/internal/propagation"
)

func (s *Scanner) decorateTrend(ctx context.Context, trend *model.Trend, entity history.Entity, cluster engine.Cluster, now time.Time) error {
	if trend == nil {
		return nil
	}
	cluster = s.enrichSourceMetadata(cluster)
	trend.Sources = uniqueSources(cluster.Signals)
	trend.TopVoices = topVoices(cluster.Signals, 8)
	trend.Perspective = perspective.Analyze(trend.Sources)

	currentPath := propagation.Build(cluster.Signals, now)
	if err := s.history.RecordPropagation(ctx, entity.ID, currentPath, now); err != nil {
		return err
	}
	path, err := s.history.Propagation(ctx, entity.ID)
	if err != nil {
		return err
	}
	// Bias/source metadata is a catalog property and is not duplicated into the
	// propagation history table. Rehydrate it when serving the current object.
	for index := range path {
		if catalog, ok := s.memory.Source(path[index].Source.Domain); ok {
			path[index].Source = catalog
		}
	}
	trend.Propagation = path

	explanation := explain.Build(*trend, cluster.Signals, entity.FirstSeen)
	trend.Explanation = &explanation
	trend.Why = explanation.Summary
	trend.Lore = explanation.Lore
	return nil
}

func (s *Scanner) enrichSourceMetadata(cluster engine.Cluster) engine.Cluster {
	out := cluster
	out.Signals = append([]model.Signal(nil), cluster.Signals...)
	for index := range out.Signals {
		domain := out.Signals[index].Source.Domain
		if domain == "" {
			continue
		}
		if catalog, ok := s.memory.Source(domain); ok {
			out.Signals[index].Source = catalog
		}
	}
	return out
}

func topVoices(values []model.Signal, limit int) []model.ActorProfile {
	if limit <= 0 {
		limit = 8
	}
	type voice struct {
		profile model.ActorProfile
		weight  int
		key     string
	}
	seen := map[string]voice{}
	for _, signal := range values {
		if signal.Actor == nil {
			continue
		}
		key := signal.Actor.DID
		if key == "" {
			key = signal.Actor.Handle
		}
		if key == "" {
			continue
		}
		weight := signal.Engagement.Score + signal.Engagement.Likes + signal.Engagement.Replies +
			2*signal.Engagement.Reposts + 2*signal.Engagement.Quotes
		current, ok := seen[key]
		if !ok || weight > current.weight {
			seen[key] = voice{profile: *signal.Actor, weight: weight, key: key}
		}
	}
	voices := make([]voice, 0, len(seen))
	for _, value := range seen {
		voices = append(voices, value)
	}
	sort.SliceStable(voices, func(i, j int) bool {
		if voices[i].weight == voices[j].weight {
			return voices[i].key < voices[j].key
		}
		return voices[i].weight > voices[j].weight
	})
	if len(voices) > limit {
		voices = voices[:limit]
	}
	out := make([]model.ActorProfile, 0, len(voices))
	for _, value := range voices {
		out = append(out, value.profile)
	}
	return out
}
