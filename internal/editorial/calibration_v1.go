package editorial

import (
	"context"
	"fmt"
	"math"

	qualityeval "github.com/chrisbirster/trendinary/internal/eval"
)

const calibrationRequiredLabels = 100
const calibrationRequiredReplaySignals = 50
const currentClusterThreshold = 0.42

type CalibrationCandidate struct {
	Threshold float64            `json:"threshold"`
	F1        float64            `json:"f1"`
	Replay    qualityeval.Report `json:"replay"`
}

type CalibrationV1 struct {
	Ready                       bool                  `json:"ready"`
	RequiredLabels              int                   `json:"required_labels"`
	Labels                      int                   `json:"labels"`
	RequiredReplaySignals       int                   `json:"required_replay_signals"`
	ReplaySignals               int                   `json:"replay_signals"`
	CurrentClusterThreshold     float64               `json:"current_cluster_threshold"`
	RecommendedClusterThreshold *float64              `json:"recommended_cluster_threshold,omitempty"`
	RecommendedMinScore         int                   `json:"recommended_min_score"`
	CurrentReplay               qualityeval.Report    `json:"current_replay"`
	BestReplay                  *CalibrationCandidate `json:"best_replay,omitempty"`
	Note                        string                `json:"note"`
}

// CalibrationV1 refuses to recommend a clustering threshold until enough
// distinct human-labeled trends and persisted replay evidence exist. Before
// that point it reports progress only; production behavior is never tuned from
// a tiny or membership-empty sample.
func (s *Store) CalibrationV1(ctx context.Context) (CalibrationV1, error) {
	report, err := s.QualityReport(ctx)
	if err != nil {
		return CalibrationV1{}, err
	}
	current, err := s.QualityReplay(ctx, currentClusterThreshold)
	if err != nil {
		return CalibrationV1{}, err
	}
	readyLabels := report.Labels >= calibrationRequiredLabels
	readyReplay := current.Signals >= calibrationRequiredReplaySignals
	value := CalibrationV1{
		Ready:                   readyLabels && readyReplay,
		RequiredLabels:          calibrationRequiredLabels,
		Labels:                  report.Labels,
		RequiredReplaySignals:   calibrationRequiredReplaySignals,
		ReplaySignals:           current.Signals,
		CurrentClusterThreshold: currentClusterThreshold,
		RecommendedMinScore:     report.RecommendedMinScore,
		CurrentReplay:           current,
	}
	if !value.Ready {
		switch {
		case !readyLabels && !readyReplay:
			value.Note = fmt.Sprintf("Collect %d more distinct human-labeled trends and at least %d persisted replay signals before tuning thresholds.", max(0, calibrationRequiredLabels-report.Labels), calibrationRequiredReplaySignals)
		case !readyLabels:
			value.Note = fmt.Sprintf("Collect %d more distinct human-labeled trends before tuning thresholds.", calibrationRequiredLabels-report.Labels)
		default:
			value.Note = fmt.Sprintf("Human label count is sufficient, but replay has only %d/%d persisted signals; wait for labeled trend memberships to accumulate before tuning.", current.Signals, calibrationRequiredReplaySignals)
		}
		return value, nil
	}

	var best *CalibrationCandidate
	for threshold := 0.30; threshold <= 0.7001; threshold += 0.02 {
		candidateThreshold := math.Round(threshold*100) / 100
		replay, err := s.QualityReplay(ctx, candidateThreshold)
		if err != nil {
			return CalibrationV1{}, err
		}
		candidate := CalibrationCandidate{Threshold: candidateThreshold, F1: replayF1(replay), Replay: replay}
		if best == nil || candidate.F1 > best.F1 || (candidate.F1 == best.F1 && candidate.Replay.Precision > best.Replay.Precision) {
			copy := candidate
			best = &copy
		}
	}
	if best != nil {
		threshold := best.Threshold
		value.RecommendedClusterThreshold = &threshold
		value.BestReplay = best
	}
	value.Note = "Recommendation is replay-derived from the latest human label per stable trend; changing production thresholds still requires a reviewed code change."
	return value, nil
}

func replayF1(report qualityeval.Report) float64 {
	if report.Precision+report.Recall == 0 {
		return 0
	}
	return 2 * report.Precision * report.Recall / (report.Precision + report.Recall)
}
