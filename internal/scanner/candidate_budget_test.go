package scanner

import "testing"

func TestTopCandidateProcessingLimitIncludesBoundedReplacementReserve(t *testing.T) {
	if got := topCandidateProcessingLimit(20); got != 25 {
		t.Fatalf("default Top 20 processing limit = %d, want 25", got)
	}
	if got := topCandidateProcessingLimit(10); got != 15 {
		t.Fatalf("Top 10 processing limit = %d, want 15", got)
	}
	if got := topCandidateProcessingLimit(150); got != maxCandidateClusters {
		t.Fatalf("large processing limit = %d, want cap %d", got, maxCandidateClusters)
	}
}
