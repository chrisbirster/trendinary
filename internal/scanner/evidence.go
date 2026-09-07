package scanner

import (
	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

// relatedEvidence keeps historical membership useful without trusting it
// blindly. A historical observation must agree with a strong majority of the
// current cluster, not merely one member. This prevents one bridge headline
// from pulling old unrelated evidence back into a stable public trend.
func relatedEvidence(current, historical []model.Signal, threshold float64) []model.Signal {
	if len(current) == 0 || len(historical) == 0 {
		return nil
	}
	requiredMatches := (2*len(current) + 2) / 3 // ceil(2n/3)
	if requiredMatches < 1 {
		requiredMatches = 1
	}
	out := make([]model.Signal, 0, len(historical))
	for _, candidate := range historical {
		matches := 0
		for _, live := range current {
			if engine.SameEvent(live, candidate, threshold) {
				matches++
			}
		}
		if matches >= requiredMatches {
			out = append(out, candidate)
		}
	}
	return deduplicateSignals(out)
}
