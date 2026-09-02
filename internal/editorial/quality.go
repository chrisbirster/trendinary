package editorial

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	qualityeval "github.com/chrisbirster/trendinary/internal/eval"
	"github.com/chrisbirster/trendinary/internal/model"
)

type QualityLabel string

const (
	QualityRealTrend  QualityLabel = "real-trend"
	QualityNoise      QualityLabel = "noise"
	QualityDuplicate  QualityLabel = "duplicate"
	QualityTooEarly   QualityLabel = "interesting-too-early"
	QualityTooLate    QualityLabel = "detected-too-late"
	QualityBadCluster QualityLabel = "bad-cluster"
	QualityWrongName  QualityLabel = "wrong-canonical-name"
)

var qualitySchemaReady sync.Map

const qualitySchema = `
CREATE TABLE IF NOT EXISTS quality_feedback (
  id TEXT PRIMARY KEY,
  trend_key TEXT NOT NULL,
  trend_slug TEXT NOT NULL,
  trend_name TEXT NOT NULL,
  label TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  score INTEGER NOT NULL DEFAULT 0,
  lifecycle TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_quality_feedback_trend_time
  ON quality_feedback(trend_key, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_quality_feedback_label_time
  ON quality_feedback(label, created_at DESC);
CREATE TABLE IF NOT EXISTS trend_signal_memberships (
  trend_key TEXT NOT NULL,
  signal_id TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY (trend_key, signal_id)
);
CREATE INDEX IF NOT EXISTS idx_trend_signal_memberships_trend
  ON trend_signal_memberships(trend_key, observed_at DESC);
`

type QualityFeedback struct {
	ID        string       `json:"id"`
	TrendKey  string       `json:"trend_key"`
	TrendSlug string       `json:"trend_slug"`
	TrendName string       `json:"trend_name"`
	Label     QualityLabel `json:"label"`
	Note      string       `json:"note,omitempty"`
	Score     int          `json:"score"`
	Lifecycle string       `json:"lifecycle,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
}

type QualityReport struct {
	Labels                         int     `json:"labels"`
	RealTrends                     int     `json:"real_trends"`
	Noise                          int     `json:"noise"`
	Duplicates                     int     `json:"duplicates"`
	InterestingTooEarly            int     `json:"interesting_too_early"`
	DetectedTooLate                int     `json:"detected_too_late"`
	BadClusters                    int     `json:"bad_clusters"`
	WrongNames                     int     `json:"wrong_names"`
	PrecisionProxy                 float64 `json:"precision_proxy"`
	Top10Precision                 float64 `json:"top_10_precision"`
	Top10Evaluated                 int     `json:"top_10_evaluated"`
	Top25Precision                 float64 `json:"top_25_precision"`
	Top25Evaluated                 int     `json:"top_25_evaluated"`
	FalsePositiveRate              float64 `json:"false_positive_rate"`
	DuplicateClusterRate           float64 `json:"duplicate_cluster_rate"`
	EarlyHitRate                   float64 `json:"early_hit_rate"`
	ClusterHealth                  float64 `json:"cluster_health"`
	NamingHealth                   float64 `json:"naming_health"`
	RecommendedMinScore            int     `json:"recommended_min_score"`
	PositiveMeanScore              float64 `json:"positive_mean_score"`
	NoiseMeanScore                 float64 `json:"noise_mean_score"`
	AverageSourceBreadth            float64 `json:"average_source_breadth"`
	AverageSourceCount              float64 `json:"average_source_count"`
	AverageLeadToBreakingMinutes    float64 `json:"average_lead_to_breaking_minutes"`
	AverageEmergingToRisingMinutes  float64 `json:"average_emerging_to_rising_minutes"`
	AverageRisingToBreakingMinutes  float64 `json:"average_rising_to_breaking_minutes"`
}

type qualityObservation struct {
	Label QualityLabel
	Score int
}

type trendTiming struct {
	TrendKey          string
	FirstSeen         time.Time
	FirstEmerging     time.Time
	FirstRising       time.Time
	FirstBreaking     time.Time
	LatestBreadth     float64
	LatestSourceCount int
	HasSnapshot       bool
}

func (s *Store) ensureQualitySchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("editorial store is unavailable")
	}
	if _, ok := qualitySchemaReady.Load(s); ok {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, qualitySchema); err != nil {
		return fmt.Errorf("ensure quality schema: %w", err)
	}
	qualitySchemaReady.Store(s, struct{}{})
	return nil
}

func validQualityLabel(label QualityLabel) bool {
	switch label {
	case QualityRealTrend, QualityNoise, QualityDuplicate, QualityTooEarly, QualityTooLate, QualityBadCluster, QualityWrongName:
		return true
	default:
		return false
	}
}

func (s *Store) PutQualityFeedback(ctx context.Context, value QualityFeedback) (QualityFeedback, error) {
	if err := s.ensureQualitySchema(ctx); err != nil {
		return QualityFeedback{}, err
	}
	value.TrendKey = strings.TrimSpace(value.TrendKey)
	value.TrendSlug = strings.TrimSpace(value.TrendSlug)
	value.TrendName = strings.TrimSpace(value.TrendName)
	value.Note = strings.TrimSpace(value.Note)
	if value.TrendKey == "" {
		value.TrendKey = value.TrendSlug
	}
	if value.TrendKey == "" || value.TrendSlug == "" {
		return QualityFeedback{}, fmt.Errorf("trend key and slug are required")
	}
	if !validQualityLabel(value.Label) {
		return QualityFeedback{}, fmt.Errorf("unsupported quality label %q", value.Label)
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	if value.ID == "" {
		value.ID = fmt.Sprintf("quality-%s-%d", value.TrendKey, value.CreatedAt.UnixNano())
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO quality_feedback (
 id, trend_key, trend_slug, trend_name, label, note, score, lifecycle, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.TrendKey, value.TrendSlug, value.TrendName, string(value.Label),
		value.Note, value.Score, value.Lifecycle, value.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return QualityFeedback{}, fmt.Errorf("record quality feedback: %w", err)
	}
	return value, nil
}

func (s *Store) QualityFeedback(ctx context.Context, limit int) ([]QualityFeedback, error) {
	if err := s.ensureQualitySchema(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, trend_key, trend_slug, trend_name, label, note, score, lifecycle, created_at
FROM quality_feedback
ORDER BY created_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]QualityFeedback, 0, limit)
	for rows.Next() {
		var value QualityFeedback
		var label, createdAt string
		if err := rows.Scan(&value.ID, &value.TrendKey, &value.TrendSlug, &value.TrendName, &label, &value.Note, &value.Score, &value.Lifecycle, &createdAt); err != nil {
			return nil, err
		}
		value.Label = QualityLabel(label)
		parsed, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		value.CreatedAt = parsed
		out = append(out, value)
	}
	return out, rows.Err()
}

func (s *Store) QualityReport(ctx context.Context) (QualityReport, error) {
	if err := s.ensureQualitySchema(ctx); err != nil {
		return QualityReport{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT label, score
FROM quality_feedback f
WHERE id = (
  SELECT f2.id
  FROM quality_feedback f2
  WHERE f2.trend_key = f.trend_key
  ORDER BY f2.created_at DESC, f2.id DESC
  LIMIT 1
)
ORDER BY score DESC, created_at DESC`)
	if err != nil {
		return QualityReport{}, err
	}
	defer rows.Close()
	var report QualityReport
	var positiveScores, noiseScores []int
	observations := make([]qualityObservation, 0, 64)
	for rows.Next() {
		var label QualityLabel
		var score int
		if err := rows.Scan(&label, &score); err != nil {
			return QualityReport{}, err
		}
		observations = append(observations, qualityObservation{Label: label, Score: score})
		report.Labels++
		switch label {
		case QualityRealTrend:
			report.RealTrends++
			positiveScores = append(positiveScores, score)
		case QualityNoise:
			report.Noise++
			noiseScores = append(noiseScores, score)
		case QualityDuplicate:
			report.Duplicates++
		case QualityTooEarly:
			report.InterestingTooEarly++
			positiveScores = append(positiveScores, score)
		case QualityTooLate:
			report.DetectedTooLate++
			positiveScores = append(positiveScores, score)
		case QualityBadCluster:
			report.BadClusters++
		case QualityWrongName:
			report.WrongNames++
		}
	}
	if err := rows.Err(); err != nil {
		return QualityReport{}, err
	}

	positive := report.RealTrends + report.InterestingTooEarly + report.DetectedTooLate
	falsePositive := report.Noise + report.Duplicates + report.BadClusters
	if denominator := positive + falsePositive; denominator > 0 {
		report.PrecisionProxy = float64(positive) / float64(denominator)
		report.FalsePositiveRate = float64(falsePositive) / float64(denominator)
	}
	report.Top10Precision, report.Top10Evaluated = rankedPrecision(observations, 10)
	report.Top25Precision, report.Top25Evaluated = rankedPrecision(observations, 25)
	if denominator := report.InterestingTooEarly + report.DetectedTooLate; denominator > 0 {
		report.EarlyHitRate = float64(report.InterestingTooEarly) / float64(denominator)
	}
	if report.Labels > 0 {
		report.DuplicateClusterRate = float64(report.Duplicates) / float64(report.Labels)
		report.ClusterHealth = 1 - float64(report.Duplicates+report.BadClusters)/float64(report.Labels)
		report.NamingHealth = 1 - float64(report.WrongNames)/float64(report.Labels)
	}
	report.PositiveMeanScore = meanScores(positiveScores)
	report.NoiseMeanScore = meanScores(noiseScores)
	report.RecommendedMinScore = recommendedMinScore(report.PositiveMeanScore, report.NoiseMeanScore, len(positiveScores), len(noiseScores))

	if err := s.applyHistoricalQualityMetrics(ctx, &report); err != nil {
		return QualityReport{}, err
	}
	return report, nil
}

func rankedPrecision(values []qualityObservation, limit int) (float64, int) {
	if limit <= 0 {
		return 0, 0
	}
	positives := 0
	evaluated := 0
	for _, value := range values {
		positive, negative := precisionOutcome(value.Label)
		if !positive && !negative {
			continue
		}
		evaluated++
		if positive {
			positives++
		}
		if evaluated == limit {
			break
		}
	}
	if evaluated == 0 {
		return 0, 0
	}
	return float64(positives) / float64(evaluated), evaluated
}

func precisionOutcome(label QualityLabel) (positive bool, negative bool) {
	switch label {
	case QualityRealTrend, QualityTooEarly, QualityTooLate:
		return true, false
	case QualityNoise, QualityDuplicate, QualityBadCluster:
		return false, true
	default:
		return false, false
	}
}

func (s *Store) applyHistoricalQualityMetrics(ctx context.Context, report *QualityReport) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT s.trend_key, s.observed_at, s.lifecycle, s.source_breadth, s.source_count
FROM trend_snapshots s
JOIN quality_feedback f ON f.trend_key = s.trend_key
WHERE f.id = (
  SELECT f2.id
  FROM quality_feedback f2
  WHERE f2.trend_key = f.trend_key
  ORDER BY f2.created_at DESC, f2.id DESC
  LIMIT 1
)
AND f.label IN (?, ?, ?)
ORDER BY s.trend_key, s.observed_at`, string(QualityRealTrend), string(QualityTooEarly), string(QualityTooLate))
	if err != nil {
		// Editorial-only test stores and early development databases may not have
		// history enabled yet. Ranked label metrics are still useful without it.
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return fmt.Errorf("quality history query: %w", err)
	}
	defer rows.Close()

	var current trendTiming
	var trendCount int
	var breadthTotal float64
	var sourceCountTotal float64
	var leadToBreaking []time.Duration
	var emergingToRising []time.Duration
	var risingToBreaking []time.Duration

	finalize := func(value trendTiming) {
		if !value.HasSnapshot {
			return
		}
		trendCount++
		breadthTotal += value.LatestBreadth
		sourceCountTotal += float64(value.LatestSourceCount)
		if !value.FirstSeen.IsZero() && !value.FirstBreaking.IsZero() && !value.FirstBreaking.Before(value.FirstSeen) {
			leadToBreaking = append(leadToBreaking, value.FirstBreaking.Sub(value.FirstSeen))
		}
		if !value.FirstEmerging.IsZero() && !value.FirstRising.IsZero() && !value.FirstRising.Before(value.FirstEmerging) {
			emergingToRising = append(emergingToRising, value.FirstRising.Sub(value.FirstEmerging))
		}
		if !value.FirstRising.IsZero() && !value.FirstBreaking.IsZero() && !value.FirstBreaking.Before(value.FirstRising) {
			risingToBreaking = append(risingToBreaking, value.FirstBreaking.Sub(value.FirstRising))
		}
	}

	for rows.Next() {
		var trendKey, observedAt, lifecycle string
		var breadth float64
		var sourceCount int
		if err := rows.Scan(&trendKey, &observedAt, &lifecycle, &breadth, &sourceCount); err != nil {
			return err
		}
		observed, err := time.Parse(time.RFC3339Nano, observedAt)
		if err != nil {
			return fmt.Errorf("parse quality snapshot time: %w", err)
		}
		if current.TrendKey != "" && current.TrendKey != trendKey {
			finalize(current)
			current = trendTiming{}
		}
		if current.TrendKey == "" {
			current.TrendKey = trendKey
			current.FirstSeen = observed
		}
		current.HasSnapshot = true
		current.LatestBreadth = breadth
		current.LatestSourceCount = sourceCount
		switch strings.ToUpper(strings.TrimSpace(lifecycle)) {
		case "EMERGING":
			if current.FirstEmerging.IsZero() {
				current.FirstEmerging = observed
			}
		case "RISING":
			if current.FirstRising.IsZero() {
				current.FirstRising = observed
			}
		case "BREAKING":
			if current.FirstBreaking.IsZero() {
				current.FirstBreaking = observed
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	finalize(current)

	if trendCount > 0 {
		report.AverageSourceBreadth = breadthTotal / float64(trendCount)
		report.AverageSourceCount = sourceCountTotal / float64(trendCount)
	}
	report.AverageLeadToBreakingMinutes = averageMinutes(leadToBreaking)
	report.AverageEmergingToRisingMinutes = averageMinutes(emergingToRising)
	report.AverageRisingToBreakingMinutes = averageMinutes(risingToBreaking)
	return nil
}

func averageMinutes(values []time.Duration) float64 {
	if len(values) == 0 {
		return 0
	}
	var total time.Duration
	for _, value := range values {
		total += value
	}
	return total.Minutes() / float64(len(values))
}

func meanScores(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0
	for _, value := range values {
		total += value
	}
	return float64(total) / float64(len(values))
}

func recommendedMinScore(positiveMean, noiseMean float64, positives, negatives int) int {
	value := 35.0
	switch {
	case positives > 0 && negatives > 0:
		value = (positiveMean + noiseMean) / 2
	case positives > 0:
		value = positiveMean - 10
	case negatives > 0:
		value = noiseMean + 5
	}
	value = math.Max(20, math.Min(80, value))
	return int(math.Round(value))
}

// QualityReplay converts the latest human trend labels plus persisted signal
// memberships into the deterministic pairwise clustering benchmark used by the
// existing replay evaluator. Positive labels share their stable trend key;
// negative/structural labels deliberately receive unique groups.
func (s *Store) QualityReplay(ctx context.Context, threshold float64) (qualityeval.Report, error) {
	if err := s.ensureQualitySchema(ctx); err != nil {
		return qualityeval.Report{}, err
	}
	if threshold <= 0 {
		threshold = 0.56
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT f.trend_key, f.label,
       sig.id, sig.source_name, sig.source_domain, sig.title, sig.body, sig.url,
       sig.author, sig.published_at, sig.score, sig.replies, sig.likes, sig.reposts, sig.quotes
FROM quality_feedback f
JOIN trend_signal_memberships m ON m.trend_key = f.trend_key
JOIN signals sig ON sig.id = m.signal_id
WHERE f.created_at = (
  SELECT MAX(f2.created_at) FROM quality_feedback f2 WHERE f2.trend_key = f.trend_key
)
ORDER BY f.trend_key, m.observed_at
LIMIT 2500`)
	if err != nil {
		return qualityeval.Report{}, fmt.Errorf("quality replay query: %w", err)
	}
	defer rows.Close()
	corpus := qualityeval.Corpus{Name: "trendinary-human-quality-v3"}
	for rows.Next() {
		var trendKey, label, sourceName, sourceDomain, title, body, rawURL, author string
		var published sql.NullString
		var signal model.Signal
		if err := rows.Scan(
			&trendKey, &label,
			&signal.ID, &sourceName, &sourceDomain, &title, &body, &rawURL,
			&author, &published, &signal.Engagement.Score, &signal.Engagement.Replies,
			&signal.Engagement.Likes, &signal.Engagement.Reposts, &signal.Engagement.Quotes,
		); err != nil {
			return qualityeval.Report{}, err
		}
		signal.Source = model.Source{Name: sourceName, Domain: sourceDomain}
		signal.Title = title
		signal.Text = body
		signal.URL = rawURL
		signal.Author = author
		if published.Valid {
			signal.PublishedAt = published.String
		}
		group := trendKey
		switch QualityLabel(label) {
		case QualityRealTrend, QualityTooEarly, QualityTooLate:
		default:
			group = "negative:" + signal.ID
		}
		corpus.Signals = append(corpus.Signals, qualityeval.LabeledSignal{Signal: signal, Group: group})
	}
	if err := rows.Err(); err != nil {
		return qualityeval.Report{}, err
	}
	if len(corpus.Signals) == 0 {
		return qualityeval.Report{Corpus: corpus.Name}, nil
	}
	return qualityeval.Evaluate(corpus, threshold), nil
}
