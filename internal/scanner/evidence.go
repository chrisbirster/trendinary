package scanner

import (
	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

// relatedEvidence keeps historical membership useful without trusting it
// blindly. Every persisted signal must still match at least one current signal
// under the current event resolver before it can affect source breadth or the
// public explanation.
func relatedEvidence(current, historical []model.Signal, threshold float64) []model.Signal {
	if len(current) == 0 || len(historical) == 0 {
		return nil
	}
	out := make([]model.Signal, 0, len(historical))
	for _, candidate := range historical {
		for _, live := range current {
			if engine.SameEvent(live, candidate, threshold) {
				out = append(out, candidate)
				break
			}
		}
	}
	return deduplicateSignals(out)
}
