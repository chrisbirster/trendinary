package scanner

import "testing"

func TestTopCandidateProcessingLimitMatchesPublishCapacity(t *testing.T) {
	if got := topCandidateProcessingLimit(20); got != 20 {
		t.Fatalf("default Top 20 processing limit = %d, want 20", got)
	}
	if got := topCandidateProcessingLimit(10); got != 10 {
		t.Fatalf("Top 10 processing limit = %d, want 10", got)
	}
	if got := topCandidateProcessingLimit(150); got != maxCandidateClusters {
		t.Fatalf("large processing limit = %d, want cap %d", got, maxCandidateClusters)
	}
}
