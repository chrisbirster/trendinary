package scanner

// topCandidateProcessingLimit bounds expensive durable scoring work relative to
// the number of trends we can actually publish. The pre-score candidate weight
// still considers every discovered cluster; only the strongest shortlist pays
// for entity resolution, rolling evidence, baselines, propagation, snapshots,
// and chart history over remote libSQL.
func topCandidateProcessingLimit(published int) int {
	if published <= 0 {
		published = 20
	}
	limit := published * 2
	if limit > maxCandidateClusters {
		return maxCandidateClusters
	}
	return limit
}
