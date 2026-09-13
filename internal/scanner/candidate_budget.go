package scanner

// topCandidateProcessingLimit bounds remote durable scoring work to the number
// of trends we can actually publish. The full discovered cluster universe still
// competes in the cheap pre-score ranking before this shortlist is chosen.
func topCandidateProcessingLimit(published int) int {
	if published <= 0 {
		published = 20
	}
	if published > maxCandidateClusters {
		return maxCandidateClusters
	}
	return published
}
