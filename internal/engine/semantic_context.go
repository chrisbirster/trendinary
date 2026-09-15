package engine

import (
	"context"
	"sort"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type preparedSignal struct {
	signal    model.Signal
	terms     map[string]struct{}
	entities  map[string]struct{}
	published time.Time
	hasTime   bool
}

type preparedCluster struct {
	signals []model.Signal
	members []preparedSignal
}

// ClusterSignalsV2Context is the cancellable scanner-facing variant of
// ClusterSignalsV2. It preserves the same deterministic complete-link behavior
// while precomputing semantic features once per signal. The original SameEvent
// path tokenizes text and allocates a Jaccard union for every pairwise
// comparison; doing that across a large discovery batch made scanner runtime
// grow dramatically. This path keeps pairwise comparisons allocation-free and
// checks the caller's deadline throughout the clustering loops.
func ClusterSignalsV2Context(ctx context.Context, input []model.Signal, threshold float64) ([]Cluster, error) {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.42
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	values := append([]model.Signal(nil), input...)
	sort.SliceStable(values, func(i, j int) bool {
		left, right := signalSortKey(values[i]), signalSortKey(values[j])
		return left < right
	})

	prepared := make([]preparedSignal, len(values))
	for index, signal := range values {
		if index%32 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		published, hasTime := parseSignalTime(signal.PublishedAt)
		prepared[index] = preparedSignal{
			signal:    signal,
			terms:     SignalTerms(signal),
			entities:  EntityKeys(signal),
			published: published,
			hasTime:   hasTime,
		}
	}

	clusters := make([]preparedCluster, 0, len(prepared))
	for signalIndex, signal := range prepared {
		if signalIndex%16 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		placed := false
		for clusterIndex := range clusters {
			if clusterIndex%32 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if preparedMatchesEntireCluster(signal, clusters[clusterIndex], threshold) {
				clusters[clusterIndex].signals = append(clusters[clusterIndex].signals, signal.signal)
				clusters[clusterIndex].members = append(clusters[clusterIndex].members, signal)
				placed = true
				break
			}
		}
		if !placed {
			clusters = append(clusters, preparedCluster{
				signals: []model.Signal{signal.signal},
				members: []preparedSignal{signal},
			})
		}
	}

	result := make([]Cluster, len(clusters))
	for index := range clusters {
		if index%32 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		result[index] = Cluster{
			Key:     clusterKey(clusters[index].signals),
			Signals: clusters[index].signals,
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if len(result[i].Signals) == len(result[j].Signals) {
			return result[i].Key < result[j].Key
		}
		return len(result[i].Signals) > len(result[j].Signals)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func preparedMatchesEntireCluster(signal preparedSignal, cluster preparedCluster, threshold float64) bool {
	if len(cluster.members) == 0 {
		return true
	}
	for _, member := range cluster.members {
		if !samePreparedEvent(signal, member, threshold) {
			return false
		}
	}
	return true
}

func samePreparedEvent(a, b preparedSignal, lexicalThreshold float64) bool {
	lexical := similarityNoAlloc(a.terms, b.terms)
	if lexical >= lexicalThreshold {
		return true
	}
	overlap := overlapCoefficient(a.terms, b.terms)
	entity := similarityNoAlloc(a.entities, b.entities)
	sharedEntity := hasSharedKey(a.entities, b.entities)
	proximity := preparedTimeProximity(a, b)

	score := lexical*.25 + overlap*.40 + entity*.25 + proximity*.10
	if sharedEntity && overlap >= .20 {
		score += .15
	}
	if score > 1 {
		score = 1
	}
	return score >= .52 && (overlap >= .20 || entity >= .34)
}

func similarityNoAlloc(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for token := range a {
		if _, ok := b[token]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func preparedTimeProximity(a, b preparedSignal) float64 {
	if !a.hasTime || !b.hasTime {
		return 0
	}
	delta := a.published.Sub(b.published)
	if delta < 0 {
		delta = -delta
	}
	switch {
	case delta <= 2*time.Hour:
		return 1
	case delta <= 6*time.Hour:
		return .8
	case delta <= 24*time.Hour:
		return .55
	case delta <= 72*time.Hour:
		return .2
	default:
		return 0
	}
}
