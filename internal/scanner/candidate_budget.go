package scanner

const candidateReplacementReserve = 5

// topCandidateProcessingLimit keeps a small bounded reserve beyond the public
// chart size so an identity/score/snapshot failure cannot turn a healthy Top 20
// candidate pool into an 18- or 19-item chart. This remains intentionally tiny:
// discovery and clustering are cheap/in-memory, while durable scoring work stays
// bounded to publish capacity plus a handful of replacements.
func topCandidateProcessingLimit(published int) int {
	if published <= 0 {
		published = 20
	}
	limit := published + candidateReplacementReserve
	if limit > maxCandidateClusters {
		return maxCandidateClusters
	}
	return limit
}
