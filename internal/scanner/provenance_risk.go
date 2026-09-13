package scanner

import (
	"strings"

	"github.com/chrisbirster/trendinary/internal/model"
)

// scoringProvenance discounts publisher breadth when every independent signal
// comes from GitHub. Separate repository owners can be genuinely independent,
// but GitHub search is also unusually easy for coordinated repository bursts to
// game. Keep the full publisher list for UI provenance while requiring another
// platform before multiple GitHub owners earn corroboration breadth.
func scoringProvenance(value model.TrendProvenance) model.TrendProvenance {
	if githubOnlyProvenance(value) && value.PublisherCount > 1 {
		value.PublisherCount = 1
	}
	return value
}

func githubOnlyProvenance(value model.TrendProvenance) bool {
	if value.PlatformCount != 1 || len(value.Platforms) != 1 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(value.Platforms[0]), "GitHub")
}
