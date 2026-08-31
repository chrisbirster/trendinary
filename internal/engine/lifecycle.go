package engine

type LifecycleInput struct {
	Score          int     `json:"score"`
	PreviousScore  int     `json:"previous_score"`
	Velocity       float64 `json:"velocity"`
	SourceBreadth  float64 `json:"source_breadth"`
	PreviouslyCold bool    `json:"previously_cold"`
}

// Lifecycle converts score movement into product language. The thresholds are
// intentionally explicit and versionable; they are a starting point that can
// later be calibrated against historical outcomes.
func Lifecycle(input LifecycleInput) string {
	delta := input.Score - input.PreviousScore

	if input.PreviouslyCold && input.Score >= 55 && input.Velocity >= 0.65 {
		return "RESURFACING"
	}
	if input.Score >= 85 && input.Velocity >= 0.75 && input.SourceBreadth >= 0.6 {
		return "BREAKING"
	}
	if input.Score >= 75 && delta >= -3 && delta <= 3 && input.Velocity < 0.55 {
		return "PEAKING"
	}
	if delta <= -10 || (input.Score < 60 && input.Velocity < 0.35) {
		return "COOLING"
	}
	if input.Score >= 60 && (delta >= 5 || input.Velocity >= 0.6) {
		return "RISING"
	}
	return "EMERGING"
}
