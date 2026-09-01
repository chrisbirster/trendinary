package propagation

import (
	"sort"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

// Build summarizes how a current cluster is distributed across source
// networks. Persisting these observations over time lets Trendinary reconstruct
// origin and cross-network breakout order instead of guessing from article
// publication order on the detail page.
func Build(signals []model.Signal, fallback time.Time) []model.PropagationHop {
	if fallback.IsZero() {
		fallback = time.Now().UTC()
	}
	type aggregate struct {
		source      model.Source
		first       time.Time
		last        time.Time
		signalCount int
		engagement  int
	}
	groups := make(map[string]*aggregate)
	for _, signal := range signals {
		key := signal.Source.Domain
		if key == "" {
			key = signal.Source.Name
		}
		if key == "" {
			continue
		}
		at := publishedAt(signal, fallback)
		group := groups[key]
		if group == nil {
			group = &aggregate{source: signal.Source, first: at, last: at}
			groups[key] = group
		}
		if at.Before(group.first) {
			group.first = at
		}
		if at.After(group.last) {
			group.last = at
		}
		group.signalCount++
		group.engagement += signal.Engagement.Score + signal.Engagement.Likes +
			2*signal.Engagement.Reposts + 2*signal.Engagement.Quotes + signal.Engagement.Replies
	}

	out := make([]model.PropagationHop, 0, len(groups))
	for _, group := range groups {
		out = append(out, model.PropagationHop{
			Source:      group.source,
			FirstSeen:   group.first.UTC().Format(time.RFC3339),
			LastSeen:    group.last.UTC().Format(time.RFC3339),
			SignalCount: group.signalCount,
			Engagement:  group.engagement,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FirstSeen == out[j].FirstSeen {
			return out[i].Source.Domain < out[j].Source.Domain
		}
		return out[i].FirstSeen < out[j].FirstSeen
	})
	return out
}

func publishedAt(signal model.Signal, fallback time.Time) time.Time {
	if signal.PublishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, signal.PublishedAt); err == nil {
			return parsed.UTC()
		}
		if parsed, err := time.Parse(time.RFC3339, signal.PublishedAt); err == nil {
			return parsed.UTC()
		}
	}
	return fallback.UTC()
}
