package engine

import "math"

const ScoreVersion = "0.4"

type ScoreInput struct {
	Attention        float64 `json:"attention"`
	Velocity         float64 `json:"velocity"`
	// SourceBreadth and CommunityBreadth are compatibility aliases retained for
	// callers from score v3. In v4 the dimensions are explicitly publisher and
	// platform breadth.
	SourceBreadth    float64 `json:"source_breadth,omitempty"`
	CommunityBreadth float64 `json:"community_breadth,omitempty"`
	PublisherBreadth float64 `json:"publisher_breadth,omitempty"`
	PlatformBreadth  float64 `json:"platform_breadth,omitempty"`
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
	PublisherBreadth float64 `json:"publisher_breadth"`
	PlatformBreadth  float64 `json:"platform_breadth"`
	Novelty          float64 `json:"novelty"`
	Confidence       float64 `json:"confidence"`
}

// Score calculates Trendinary Score v4. Rank measures attention/momentum; it
// does not claim certainty. Publisher breadth and platform breadth are separate
// inputs so ten feeds from one newsroom cannot masquerade as ten independent
// publishers, and two accounts on one social platform remain distinct from
// cross-platform propagation.
func Score(input ScoreInput) ScoreBreakdown {
	publisherBreadth := input.PublisherBreadth
	if publisherBreadth == 0 && input.SourceBreadth != 0 {
		publisherBreadth = input.SourceBreadth
	}
	platformBreadth := input.PlatformBreadth
	if platformBreadth == 0 && input.CommunityBreadth != 0 {
		platformBreadth = input.CommunityBreadth
	}

	input.Attention = clamp01(input.Attention)
	input.Velocity = clamp01(input.Velocity)
	publisherBreadth = clamp01(publisherBreadth)
	platformBreadth = clamp01(platformBreadth)
	input.Novelty = clamp01(input.Novelty)
	input.Confidence = clamp01(input.Confidence)

	weighted :=
		0.14*input.Attention +
			0.32*input.Velocity +
			0.20*publisherBreadth +
			0.15*platformBreadth +
			0.11*input.Novelty +
			0.08*input.Confidence

	return ScoreBreakdown{
		Version:          ScoreVersion,
		Score:            int(math.Round(weighted * 100)),
		Attention:        input.Attention,
		Velocity:         input.Velocity,
		SourceBreadth:    publisherBreadth,
		CommunityBreadth: platformBreadth,
		PublisherBreadth: publisherBreadth,
		PlatformBreadth:  platformBreadth,
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
