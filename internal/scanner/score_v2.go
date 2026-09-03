package scanner

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
)

// scoreClusterV3 keeps instantaneous attention/velocity grounded in the current
// scan while source/community breadth comes from the related rolling evidence
// window. This prevents old signals from inflating attention but lets the UI and
// score remember that an event propagated across publishers between scans.
func (s *Scanner) scoreClusterV3(ctx context.Context, current, evidence engine.Cluster, now time.Time, calibration history.Calibration) (model.Trend, history.Snapshot, error) {
	raw := rawMetrics(current)
	evidenceRaw := rawMetrics(evidence)
	baseline, err := s.history.Baseline(ctx, current.Key, now.Add(-s.config.BaselineWindow))
	if err != nil {
		return model.Trend{}, history.Snapshot{}, err
	}
	scale := 180.0
	if calibration.Observations >= 50 && calibration.AttentionP75 > 0 {
		scale = math.Max(60, math.Min(600, calibration.AttentionP75))
	}
	attention := saturating(raw.RawAttention, scale)
	velocity := velocityScore(raw.RawAttention, baseline.AverageAttention, baseline.Observations)
	sourceBreadth := clamp01(float64(evidenceRaw.SourceCount) / float64(s.config.SourceUniverse))
	communityBreadth := clamp01(float64(evidenceRaw.CommunityCount) / 10)
	novelty := noveltyScore(baseline, now)
	confidence := clamp01(.15 + math.Min(float64(raw.SignalCount)/10, 1)*.55 + sourceBreadth*.30)
	score := engine.Score(engine.ScoreInput{
		Attention: attention, Velocity: velocity, SourceBreadth: sourceBreadth,
		CommunityBreadth: communityBreadth, Novelty: novelty, Confidence: confidence,
	})
	previousScore := 0
	previousCold := false
	if baseline.Latest != nil {
		previousScore = baseline.Latest.Score.Score
		previousCold = baseline.Latest.Score.Score < 35
	}
	thresholds := engine.CalibratedThresholds(calibration.Observations, calibration.ScoreP75, calibration.ScoreP90)
	lifecycle := engine.LifecycleWithThresholds(engine.LifecycleInput{
		Score: score.Score, PreviousScore: previousScore, Velocity: velocity,
		SourceBreadth: sourceBreadth, PreviouslyCold: previousCold,
	}, thresholds)
	trend := model.Trend{
		Slug: current.Key, Name: clusterName(current), Category: "INTERNET", Score: score.Score,
		Change: changeLabel(raw.RawAttention, baseline.AverageAttention, baseline.Observations),
		Status: lifecycle, Started: startedLabel(evidence, now), Vibe: vibeLabel(sourceBreadth, velocity, novelty),
		Reason: rollingReasonLabel(raw, evidenceRaw, baseline, velocity), Sources: uniqueSources(evidence.Signals), Timeline: timeline(evidence, now),
		Quality: qualityBreakdown(score),
	}
	snapshotRaw := raw
	snapshotRaw.SourceCount = evidenceRaw.SourceCount
	snapshotRaw.CommunityCount = evidenceRaw.CommunityCount
	snapshot := history.Snapshot{TrendKey: current.Key, ObservedAt: now, Lifecycle: lifecycle, Score: score, Raw: snapshotRaw}
	return trend, snapshot, nil
}

func rollingReasonLabel(current, evidence history.RawMetrics, baseline history.Baseline, velocity float64) string {
	if baseline.Observations == 0 {
		return fmt.Sprintf("New cluster: %d current signal(s); %d publisher source(s) observed in the rolling evidence window.", current.SignalCount, evidence.SourceCount)
	}
	ratio := current.RawAttention / math.Max(baseline.AverageAttention, 1)
	return fmt.Sprintf("%d current signal(s); %d publisher source(s) in the rolling evidence window; attention is %.1fx its recent baseline (velocity %.0f/100).", current.SignalCount, evidence.SourceCount, ratio, velocity*100)
}

func qualityBreakdown(score engine.ScoreBreakdown) model.TrendQuality {
	peep := (0.52*score.Velocity + 0.20*score.SourceBreadth + 0.13*score.CommunityBreadth + 0.15*score.Novelty) * (0.70 + 0.30*score.Confidence)
	why := make([]string, 0, 4)
	if score.Velocity >= 0.55 {
		why = append(why, fmt.Sprintf("velocity %d%%", int(math.Round(score.Velocity*100))))
	}
	if score.SourceBreadth >= 0.25 {
		why = append(why, fmt.Sprintf("source breadth %d%%", int(math.Round(score.SourceBreadth*100))))
	}
	if score.CommunityBreadth >= 0.25 {
		why = append(why, fmt.Sprintf("community spread %d%%", int(math.Round(score.CommunityBreadth*100))))
	}
	if score.Novelty >= 0.55 {
		why = append(why, fmt.Sprintf("novelty %d%%", int(math.Round(score.Novelty*100))))
	}
	if len(why) == 0 {
		why = append(why, fmt.Sprintf("attention %d%%", int(math.Round(score.Attention*100))))
	}
	return model.TrendQuality{
		Attention: score.Attention, Velocity: score.Velocity, SourceBreadth: score.SourceBreadth,
		CommunityBreadth: score.CommunityBreadth, Novelty: score.Novelty, Confidence: score.Confidence,
		PeepScore: int(math.Round(clamp01(peep) * 100)), WhyWatching: why,
	}
}
