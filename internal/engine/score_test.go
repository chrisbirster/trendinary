package engine_test

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
)

func TestScoreRewardsVelocityAndBreadth(t *testing.T) {
	result := engine.Score(engine.ScoreInput{
		Attention: 0.55, Velocity: 0.95, SourceBreadth: 0.8,
		CommunityBreadth: 0.75, Novelty: 0.85, Confidence: 0.8,
	})
	if result.Score < 75 {
		t.Fatalf("score = %d, expected strong emerging signal", result.Score)
	}
	if result.Version != engine.ScoreVersion {
		t.Fatalf("version = %q", result.Version)
	}
}

func TestScoreClampsMetrics(t *testing.T) {
	result := engine.Score(engine.ScoreInput{
		Attention: -3, Velocity: 2, SourceBreadth: 2,
		CommunityBreadth: -1, Novelty: 0.5, Confidence: 1,
	})
	if result.Velocity != 1 || result.Attention != 0 || result.SourceBreadth != 1 {
		t.Fatalf("metrics were not clamped: %+v", result)
	}
	if result.Score < 0 || result.Score > 100 {
		t.Fatalf("score out of range: %d", result.Score)
	}
}
