package engine

import (
	"context"
	"sort"

	"github.com/chrisbirster/trendinary/internal/model"
)

// ClusterSignalsV2Context is the cancellable scanner-facing variant of
// ClusterSignalsV2. It preserves the same deterministic complete-link behavior
// but checks the caller's deadline throughout the expensive clustering loops so
// a scanner timeout cannot leave the process stuck in CPU work indefinitely.
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

	clusters := make([]Cluster, 0, len(values))
	for signalIndex, signal := range values {
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
			if matchesEntireCluster(signal, clusters[clusterIndex], threshold) {
				clusters[clusterIndex].Signals = append(clusters[clusterIndex].Signals, signal)
				placed = true
				break
			}
		}
		if !placed {
			clusters = append(clusters, Cluster{Signals: []model.Signal{signal}})
		}
	}

	for index := range clusters {
		if index%32 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		clusters[index].Key = clusterKey(clusters[index].Signals)
	}
	sort.SliceStable(clusters, func(i, j int) bool {
		if len(clusters[i].Signals) == len(clusters[j].Signals) {
			return clusters[i].Key < clusters[j].Key
		}
		return len(clusters[i].Signals) > len(clusters[j].Signals)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return clusters, nil
}
