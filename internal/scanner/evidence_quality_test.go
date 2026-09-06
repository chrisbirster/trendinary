package scanner

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/model"
)

func TestRelatedEvidenceRequiresCurrentClusterCohesion(t *testing.T) {
	current := []model.Signal{
		{ID: "a", Title: "OpenAI launches Atlas browser agent", PublishedAt: "2026-09-03T10:00:00Z"},
		{ID: "b", Title: "OpenAI Atlas browser agent gets Firefox plugin support", PublishedAt: "2026-09-03T10:01:00Z"},
	}
	historical := []model.Signal{
		{ID: "good", Title: "OpenAI launches Atlas browser agent with Firefox plugin support", PublishedAt: "2026-09-03T09:59:00Z"},
		{ID: "bridge-only", Title: "Firefox browser plugin update released", PublishedAt: "2026-09-03T10:02:00Z"},
	}
	got := relatedEvidence(current, historical, 0.42)
	if len(got) != 1 || got[0].ID != "good" {
		t.Fatalf("related evidence = %+v, want only cohesive historical evidence", got)
	}
}
