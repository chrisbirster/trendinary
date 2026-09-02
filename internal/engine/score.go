package engine

import "math"

const ScoreVersion = "0.3"

type ScoreInput struct {
	Attention        float64 `json:"attention"`
	Velocity         float64 `json:"velocity"`
	SourceBreadth    float64 `json:"source_breadth"`
	CommunityBreadth float64 `json:"community_breadth"`
	Novelty          float64 `json:"novelty"`
	Confidence       float64 `json:"confidence"`
}

type ScoreBreakdown struct {
	Version          string  `json:"version"`
	Score            int     `json:"score"`
	Attention        float64 `json:"attention"`
	Velocity         float64 `json:"velocity"`
	SourceBreadth    float64 `json:"source_breadth"`
	CommunityBreadth float64 `json:"community_breadth"`
	Novelty          float64 `json:"novelty"`
	Confidence       float64 `json:"confidence"`
}

// Score calculates Trendinary Score v3. Inputs are normalized to 0..1 before
// they reach this function. V3 deliberately moves weight away from absolute
// attention and toward velocity plus independent-source breadth: Trendinary is
// trying to notice surprising movement before a popularity leaderboard would.
func Score(input ScoreInput) ScoreBreakdown {
	input.Attention = clamp01(input.Attention)
	input.Velocity = clamp01(input.Velocity)
	input.SourceBreadth = clamp01(input.SourceBreadth)
	input.CommunityBreadth = clamp01(input.CommunityBreadth)
	input.Novelty = clamp01(input.Novelty)
	input.Confidence = clamp01(input.Confidence)

	weighted :=
		0.14*input.Attention +
			0.34*input.Velocity +
			0.19*input.SourceBreadth +
			0.13*input.CommunityBreadth +
			0.12*input.Novelty +
			0.08*input.Confidence

	return ScoreBreakdown{
		Version:          ScoreVersion,
		Score:            int(math.Round(weighted * 100)),
		Attention:        input.Attention,
		Velocity:         input.Velocity,
		SourceBreadth:    input.SourceBreadth,
		CommunityBreadth: input.CommunityBreadth,
		Novelty:          input.Novelty,
		Confidence:       input.Confidence,
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
