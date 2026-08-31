package scanner

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/signals"
	"github.com/chrisbirster/trendinary/internal/store"
)

type HackerNewsClient interface {
	Top(context.Context, int) ([]hackernews.Item, error)
}

type BlueskyClient interface {
	Search(context.Context, string, int) (bluesky.SearchResponse, error)
}

type Config struct {
	HackerNewsLimit      int
	EnrichClusters       int
	BlueskyLimit         int
	PublishedTrendLimit  int
	BaselineWindow       time.Duration
	ClusterThreshold     float64
	SourceUniverse       int
}

type Result struct {
	Signals  int      `json:"signals"`
	Clusters int      `json:"clusters"`
	Trends   int      `json:"trends"`
	Warnings []string `json:"warnings,omitempty"`
}

type Scanner struct {
	config  Config
	hn      HackerNewsClient
	bluesky BlueskyClient
	history *history.Store
	memory  *store.Memory
	now     func() time.Time
}

func New(hn HackerNewsClient, bsky BlueskyClient, historical *history.Store, memory *store.Memory, config Config) *Scanner {
	if config.HackerNewsLimit <= 0 {
		config.HackerNewsLimit = 30
	}
	if config.EnrichClusters <= 0 {
		config.EnrichClusters = 8
	}
	if config.BlueskyLimit <= 0 {
		config.BlueskyLimit = 20
	}
	if config.PublishedTrendLimit <= 0 {
		config.PublishedTrendLimit = 20
	}
	if config.BaselineWindow <= 0 {
		config.BaselineWindow = 7 * 24 * time.Hour
	}
	if config.ClusterThreshold <= 0 || config.ClusterThreshold > 1 {
		config.ClusterThreshold = 0.42
	}
	if config.SourceUniverse <= 0 {
		config.SourceUniverse = 2
	}
	return &Scanner{
		config: config,
		hn: hn,
		bluesky: bsky,
		history: historical,
		memory: memory,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (s *Scanner) RunOnce(ctx context.Context) (Result, error) {
	if s.hn == nil || s.history == nil || s.memory == nil {
		return Result{}, fmt.Errorf("scanner dependencies are incomplete")
	}

	hnItems, err := s.hn.Top(ctx, s.config.HackerNewsLimit)
	if err != nil {
		return Result{}, fmt.Errorf("discover hacker news: %w", err)
	}
	hnSignals := signals.HackerNews(hnItems)
	seedClusters := engine.ClusterSignals(hnSignals, s.config.ClusterThreshold)
	sort.SliceStable(seedClusters, func(i, j int) bool {
		return clusterEngagement(seedClusters[i]) > clusterEngagement(seedClusters[j])
	})

	warnings := make([]string, 0)
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
			enriched[index].Signals = append(enriched[index].Signals, signals.Bluesky(response.Posts)...)
		}
	}

	allSignals := make([]model.Signal, 0)
	for _, cluster := range enriched {
		allSignals = append(allSignals, cluster.Signals...)
	}
	if err := s.history.RecordSignals(ctx, deduplicateSignals(allSignals)); err != nil {
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
		Signals: len(deduplicateSignals(allSignals)),
		Clusters: len(enriched),
		Trends: len(trends),
		Warnings: warnings,
	}, nil
}

func (s *Scanner) scoreCluster(ctx context.Context, cluster engine.Cluster, now time.Time) (model.Trend, history.Snapshot, error) {
	raw := rawMetrics(cluster)
	baseline, err := s.history.Baseline(ctx, cluster.Key, now.Add(-s.config.BaselineWindow))
	if err != nil {
		return model.Trend{}, history.Snapshot{}, err
	}

	attention := saturating(raw.RawAttention, 180)
	velocity := velocityScore(raw.RawAttention, baseline.AverageAttention, baseline.Observations)
	sourceBreadth := clamp01(float64(raw.SourceCount) / float64(s.config.SourceUniverse))
	communityBreadth := clamp01(float64(raw.CommunityCount) / 10.0)
	novelty := noveltyScore(baseline, now)
	confidence := clamp01(0.15 + math.Min(float64(raw.SignalCount)/10.0, 1)*0.55 + sourceBreadth*0.30)

	score := engine.Score(engine.ScoreInput{
		Attention: attention,
		Velocity: velocity,
		SourceBreadth: sourceBreadth,
		CommunityBreadth: communityBreadth,
		Novelty: novelty,
		Confidence: confidence,
	})
	previousScore := 0
	previousCold := false
	if baseline.Latest != nil {
		previousScore = baseline.Latest.Score.Score
		previousCold = baseline.Latest.Score.Score < 35
	}
	lifecycle := engine.Lifecycle(engine.LifecycleInput{
		Score: score.Score,
		PreviousScore: previousScore,
		Velocity: velocity,
		SourceBreadth: sourceBreadth,
		PreviouslyCold: previousCold,
	})

	trend := model.Trend{
		Slug: cluster.Key,
		Name: clusterName(cluster),
		Category: "INTERNET",
		Score: score.Score,
		Change: changeLabel(raw.RawAttention, baseline.AverageAttention, baseline.Observations),
		Status: lifecycle,
		Started: startedLabel(cluster, now),
		Vibe: vibeLabel(sourceBreadth, velocity, novelty),
		Reason: reasonLabel(raw, baseline, velocity),
		Sources: uniqueSources(cluster.Signals),
		Timeline: timeline(cluster, now),
	}

	snapshot := history.Snapshot{
		TrendKey: cluster.Key,
		ObservedAt: now,
		Lifecycle: lifecycle,
		Score: score,
		Raw: raw,
	}
	return trend, snapshot, nil
}

func rawMetrics(cluster engine.Cluster) history.RawMetrics {
	sourcesSeen := map[string]struct{}{}
	communities := map[string]struct{}{}
	rawEngagement := 0.0
	for _, signal := range cluster.Signals {
		sourceKey := signal.Source.Domain
		if sourceKey == "" {
			sourceKey = signal.Source.Name
		}
		if sourceKey != "" {
			sourcesSeen[sourceKey] = struct{}{}
		}
		communityKey := signal.Source.Name + ":" + signal.Author
		if signal.Author == "" {
			communityKey = signal.Source.Name
		}
		communities[communityKey] = struct{}{}
		weighted := signal.Engagement.Score + signal.Engagement.Likes +
			2*signal.Engagement.Reposts + 2*signal.Engagement.Quotes + signal.Engagement.Replies
		rawEngagement += math.Log1p(float64(max(weighted, 0)))
	}
	rawAttention := float64(len(cluster.Signals))*8 + rawEngagement*12
	return history.RawMetrics{
		SignalCount: len(cluster.Signals),
		SourceCount: len(sourcesSeen),
		CommunityCount: len(communities),
		RawAttention: rawAttention,
		RawEngagement: rawEngagement,
	}
}

func clusterEngagement(cluster engine.Cluster) int {
	total := 0
	for _, signal := range cluster.Signals {
		total += signal.Engagement.Score + signal.Engagement.Likes + signal.Engagement.Reposts + signal.Engagement.Replies
	}
	return total
}

func clusterQuery(cluster engine.Cluster) string {
	if cluster.Key != "" && cluster.Key != "unknown" {
		return strings.ReplaceAll(cluster.Key, "-", " ")
	}
	return clusterName(cluster)
}

func clusterName(cluster engine.Cluster) string {
	best := ""
	bestWeight := -1
	for _, signal := range cluster.Signals {
		candidate := strings.TrimSpace(signal.Title)
		if candidate == "" {
			candidate = strings.TrimSpace(signal.Text)
			if len(candidate) > 90 {
				candidate = strings.TrimSpace(candidate[:90]) + "…"
			}
		}
		weight := signal.Engagement.Score + signal.Engagement.Likes + signal.Engagement.Reposts + signal.Engagement.Replies
		if candidate != "" && weight > bestWeight {
			best = candidate
			bestWeight = weight
		}
	}
	if best == "" {
		return strings.ReplaceAll(cluster.Key, "-", " ")
	}
	return best
}

func uniqueSources(values []model.Signal) []model.Source {
	seen := map[string]model.Source{}
	for _, signal := range values {
		key := signal.Source.Domain
		if key == "" {
			key = signal.Source.Name
		}
		if key != "" {
			seen[key] = signal.Source
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]model.Source, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func deduplicateSignals(values []model.Signal) []model.Signal {
	seen := map[string]model.Signal{}
	for _, signal := range values {
		if signal.ID != "" {
			seen[signal.ID] = signal
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]model.Signal, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func timeline(cluster engine.Cluster, now time.Time) []model.TimelineEvent {
	values := append([]model.Signal(nil), cluster.Signals...)
	sort.SliceStable(values, func(i, j int) bool {
		return signalTime(values[i], now).Before(signalTime(values[j], now))
	})
	if len(values) > 6 {
		values = values[:6]
	}
	out := make([]model.TimelineEvent, 0, len(values))
	for _, signal := range values {
		timestamp := signalTime(signal, now)
		text := strings.TrimSpace(signal.Title)
		if text == "" {
			text = strings.TrimSpace(signal.Text)
		}
		if len(text) > 140 {
			text = text[:140] + "…"
		}
		out = append(out, model.TimelineEvent{
			Time: timestamp.Format("15:04"),
			Label: signal.Source.Name,
			Text: text,
		})
	}
	return out
}

func signalTime(signal model.Signal, fallback time.Time) time.Time {
	if signal.PublishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, signal.PublishedAt); err == nil {
			return parsed
		}
		if parsed, err := time.Parse(time.RFC3339Nano, signal.PublishedAt); err == nil {
			return parsed
		}
	}
	return fallback
}

func startedLabel(cluster engine.Cluster, now time.Time) string {
	oldest := now
	for _, signal := range cluster.Signals {
		published := signalTime(signal, now)
		if published.Before(oldest) {
			oldest = published
		}
	}
	duration := now.Sub(oldest)
	if duration < time.Hour {
		minutes := int(math.Max(1, duration.Minutes()))
		return strconv.Itoa(minutes) + "m ago"
	}
	if duration < 48*time.Hour {
		return strconv.Itoa(int(duration.Hours())) + "h ago"
	}
	return strconv.Itoa(int(duration.Hours()/24)) + "d ago"
}

func reasonLabel(raw history.RawMetrics, baseline history.Baseline, velocity float64) string {
	if baseline.Observations == 0 {
		return fmt.Sprintf("New cluster: %d signals across %d source(s) with no established Trendinary baseline yet.", raw.SignalCount, raw.SourceCount)
	}
	ratio := raw.RawAttention / math.Max(baseline.AverageAttention, 1)
	return fmt.Sprintf("%d signals across %d source(s); attention is %.1fx its recent baseline (velocity %.0f/100).", raw.SignalCount, raw.SourceCount, ratio, velocity*100)
}

func changeLabel(current, average float64, observations int) string {
	if observations == 0 || average <= 0 {
		return "NEW"
	}
	change := ((current / average) - 1) * 100
	if change >= 0 {
		return fmt.Sprintf("+%.0f%%", change)
	}
	return fmt.Sprintf("%.0f%%", change)
}

func vibeLabel(sourceBreadth, velocity, novelty float64) string {
	switch {
	case sourceBreadth >= 0.95 && velocity >= 0.7:
		return "Cross-platform, accelerating"
	case novelty >= 0.85 && velocity >= 0.65:
		return "New, fast, uncertain"
	case velocity >= 0.7:
		return "Accelerating"
	case velocity < 0.35:
		return "Cooling"
	default:
		return "Active"
	}
}

func saturating(value, scale float64) float64 {
	if value <= 0 {
		return 0
	}
	return clamp01(1 - math.Exp(-value/scale))
}

func velocityScore(current, average float64, observations int) float64 {
	if observations == 0 || average <= 0 {
		return 0.72
	}
	ratio := math.Max(current/average, 0.01)
	return clamp01(0.5 + 0.5*math.Tanh(math.Log(ratio)))
}

func noveltyScore(baseline history.Baseline, now time.Time) float64 {
	if baseline.Observations == 0 || baseline.FirstSeen.IsZero() {
		return 1
	}
	age := now.Sub(baseline.FirstSeen)
	switch {
	case age < 2*time.Hour:
		return 0.9
	case age < 24*time.Hour:
		return 0.7
	case age < 7*24*time.Hour:
		return 0.45
	default:
		return 0.2
	}
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
