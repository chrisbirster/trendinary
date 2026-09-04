package editorial

import (
	"context"
	"fmt"
	"math"

	qualityeval "github.com/chrisbirster/trendinary/internal/eval"
)

const calibrationRequiredLabels = 100
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
	CurrentClusterThreshold     float64               `json:"current_cluster_threshold"`
	RecommendedClusterThreshold *float64              `json:"recommended_cluster_threshold,omitempty"`
	RecommendedMinScore         int                   `json:"recommended_min_score"`
	CurrentReplay               qualityeval.Report    `json:"current_replay"`
	BestReplay                  *CalibrationCandidate `json:"best_replay,omitempty"`
	Note                        string                `json:"note"`
}

// CalibrationV1 refuses to recommend a clustering threshold until enough
// distinct human-labeled trends exist. Before that point it reports progress
// and current metrics only; production behavior is not tuned from a tiny sample.
func (s *Store) CalibrationV1(ctx context.Context) (CalibrationV1, error) {
	report, err := s.QualityReport(ctx)
	if err != nil {
		return CalibrationV1{}, err
	}
	current, err := s.QualityReplay(ctx, currentClusterThreshold)
	if err != nil {
		return CalibrationV1{}, err
	}
	value := CalibrationV1{
		Ready:                   report.Labels >= calibrationRequiredLabels,
		RequiredLabels:          calibrationRequiredLabels,
		Labels:                  report.Labels,
		CurrentClusterThreshold: currentClusterThreshold,
		RecommendedMinScore:     report.RecommendedMinScore,
		CurrentReplay:           current,
	}
	if !value.Ready {
		value.Note = fmt.Sprintf("Collect %d more distinct human-labeled trends before tuning thresholds.", calibrationRequiredLabels-report.Labels)
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
