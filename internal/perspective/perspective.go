package perspective

import "github.com/chrisbirster/trendinary/internal/model"

const note = "Source-level political leaning is separate from factual reliability, article stance, and whether an individual claim is true. Unrated sources are shown as unrated rather than inferred."

func Analyze(sources []model.Source) model.PerspectiveMix {
	mix := model.PerspectiveMix{TotalSources: len(sources), Note: note}
	seen := make(map[string]struct{})
	for _, source := range sources {
		key := source.Domain
		if key == "" {
			key = source.Name
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if source.Bias == nil || source.Bias.Label == model.BiasNotRated {
			mix.Unrated++
			continue
		}
		mix.RatedSources++
		switch source.Bias.Label {
		case model.BiasLeft:
			mix.Left++
		case model.BiasLeanLeft:
			mix.LeanLeft++
		case model.BiasCenter:
			mix.Center++
		case model.BiasLeanRight:
			mix.LeanRight++
		case model.BiasRight:
			mix.Right++
		case model.BiasMixed:
			mix.Mixed++
		default:
			mix.Unrated++
		}
	}
	mix.TotalSources = len(seen)
	return mix
}
