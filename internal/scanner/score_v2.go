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

// scoreClusterV3 now provides the v4 chart semantics while retaining the
// function name for compatibility with the existing scanner call path. Current
// attention/velocity comes from the live scan; publisher/platform breadth comes
// from independent rolling evidence so mirrors cannot inflate corroboration.
func (s *Scanner) scoreClusterV3(ctx context.Context, current, evidence engine.Cluster, now time.Time, calibration history.Calibration) (model.Trend, history.Snapshot, error) {
	currentIndependent := independentEvidenceSignals(current.Signals)
	evidenceIndependent := independentEvidenceSignals(evidence.Signals)
	if len(currentIndependent) == 0 {
		currentIndependent = current.Signals
	}
	if len(evidenceIndependent) == 0 {
		evidenceIndependent = evidence.Signals
	}

	raw := rawMetrics(engine.Cluster{Key: current.Key, Signals: currentIndependent})
	evidenceRaw := rawMetrics(engine.Cluster{Key: evidence.Key, Signals: evidenceIndependent})
	currentProvenance := provenanceSummary(currentIndependent)
	scoringProvenance := provenanceSummary(evidenceIndependent)
	displayProvenance := provenanceSummary(evidence.Signals)
	raw.SourceCount = currentProvenance.PublisherCount
	raw.CommunityCount = currentProvenance.PlatformCount
	evidenceRaw.SourceCount = scoringProvenance.PublisherCount
	evidenceRaw.CommunityCount = scoringProvenance.PlatformCount

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
	publisherBreadth := clamp01(float64(scoringProvenance.PublisherCount) / float64(s.config.SourceUniverse))
	platformBreadth := clamp01(float64(scoringProvenance.PlatformCount) / 5.0)
	novelty := noveltyScore(baseline, now)
	confidence := clamp01(.10 + math.Min(float64(raw.SignalCount)/8, 1)*.35 + publisherBreadth*.35 + platformBreadth*.20)
	score := engine.Score(engine.ScoreInput{
		Attention: attention, Velocity: velocity, SourceBreadth: publisherBreadth,
		CommunityBreadth: platformBreadth, Novelty: novelty, Confidence: confidence,
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
		SourceBreadth: publisherBreadth, PreviouslyCold: previousCold,
	}, thresholds)
	trend := model.Trend{
		Slug: current.Key, Name: clusterName(current), Category: "INTERNET", Score: score.Score,
		Change: changeLabel(raw.RawAttention, baseline.AverageAttention, baseline.Observations),
		Status: lifecycle, Started: startedLabel(evidence, now), Vibe: vibeLabel(publisherBreadth, velocity, novelty),
		Reason: rollingReasonLabel(raw, evidenceRaw, baseline, velocity), Sources: uniqueSources(evidence.Signals), Timeline: timeline(evidence, now),
		Quality: qualityBreakdown(score), Provenance: displayProvenance,
	}
	trend.ConfidenceTier = confidenceTier(scoringProvenance, confidence)
	snapshotRaw := raw
	snapshotRaw.SourceCount = scoringProvenance.PublisherCount
	snapshotRaw.CommunityCount = scoringProvenance.PlatformCount
	snapshot := history.Snapshot{TrendKey: current.Key, ObservedAt: now, Lifecycle: lifecycle, Score: score, Raw: snapshotRaw}
	return trend, snapshot, nil
}

func rollingReasonLabel(current, evidence history.RawMetrics, baseline history.Baseline, velocity float64) string {
	if baseline.Observations == 0 {
		return fmt.Sprintf("New chart candidate: %d current signal(s); %d independent publisher(s) across %d platform(s) in the rolling evidence window.", current.SignalCount, evidence.SourceCount, evidence.CommunityCount)
	}
	ratio := current.RawAttention / math.Max(baseline.AverageAttention, 1)
	return fmt.Sprintf("%d current signal(s); %d independent publisher(s) across %d platform(s); attention is %.1fx its recent baseline (velocity %.0f/100).", current.SignalCount, evidence.SourceCount, evidence.CommunityCount, ratio, velocity*100)
}

func qualityBreakdown(score engine.ScoreBreakdown) model.TrendQuality {
	peep := (0.52*score.Velocity + 0.20*score.SourceBreadth + 0.13*score.CommunityBreadth + 0.15*score.Novelty) * (0.70 + 0.30*score.Confidence)
	why := make([]string, 0, 4)
	if score.Velocity >= 0.55 {
		why = append(why, fmt.Sprintf("velocity %d%%", int(math.Round(score.Velocity*100))))
	}
	if score.SourceBreadth >= 0.25 {
		why = append(why, fmt.Sprintf("publisher breadth %d%%", int(math.Round(score.SourceBreadth*100))))
	}
	if score.CommunityBreadth >= 0.20 {
		why = append(why, fmt.Sprintf("platform breadth %d%%", int(math.Round(score.CommunityBreadth*100))))
	}
	if score.Novelty >= 0.55 {
		why = append(why, fmt.Sprintf("novelty %d%%", int(math.Round(score.Novelty*100))))
	}
	if len(why) == 0 {
		why = append(why, fmt.Sprintf("attention %d%%", int(math.Round(score.Attention*100))))
	}
	return model.TrendQuality{
		Attention: score.Attention, Velocity: score.Velocity, SourceBreadth: score.SourceBreadth,
		CommunityBreadth: score.CommunityBreadth, PublisherBreadth: score.SourceBreadth,
		PlatformBreadth: score.CommunityBreadth, Novelty: score.Novelty, Confidence: score.Confidence,
		PeepScore: int(math.Round(clamp01(peep) * 100)), WhyWatching: why,
	}
}
