package engine

import (
	"sort"
	"strings"
	"unicode"

	"github.com/chrisbirster/trendinary/internal/model"
)

type Cluster struct {
	Key     string         `json:"key"`
	Signals []model.Signal `json:"signals"`
}

var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"for": {}, "from": {}, "in": {}, "is": {}, "it": {}, "of": {}, "on": {}, "or": {},
	"that": {}, "the": {}, "this": {}, "to": {}, "with": {},
}

// ClusterSignals is a transparent lexical baseline for v0.1. It intentionally
// avoids an opaque embedding dependency while the data pipeline is young. Two
// signals join when their meaningful-token Jaccard similarity crosses the
// threshold. We can later add entity aliases and embeddings behind this API.
func ClusterSignals(input []model.Signal, threshold float64) []Cluster {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.45
	}
	parent := make([]int, len(input))
	for i := range parent {
		parent[i] = i
	}

	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	tokens := make([]map[string]struct{}, len(input))
	for i, signal := range input {
		tokens[i] = tokenSet(signal.Title + " " + signal.Text)
	}
	for i := 0; i < len(input); i++ {
		for j := i + 1; j < len(input); j++ {
			if similarity(tokens[i], tokens[j]) >= threshold {
				union(i, j)
			}
		}
	}

	groups := map[int][]model.Signal{}
	for i, signal := range input {
		root := find(i)
		groups[root] = append(groups[root], signal)
	}

	clusters := make([]Cluster, 0, len(groups))
	for _, signals := range groups {
		clusters = append(clusters, Cluster{Key: clusterKey(signals), Signals: signals})
	}
	sort.SliceStable(clusters, func(i, j int) bool {
		if len(clusters[i].Signals) == len(clusters[j].Signals) {
			return clusters[i].Key < clusters[j].Key
		}
		return len(clusters[i].Signals) > len(clusters[j].Signals)
	})
	return clusters
}

func tokenSet(text string) map[string]struct{} {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	set := make(map[string]struct{}, len(words))
	for _, word := range words {
		if len(word) < 2 {
			continue
		}
		if _, blocked := stopWords[word]; blocked {
			continue
		}
		set[word] = struct{}{}
	}
	return set
}

func similarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	union := make(map[string]struct{}, len(a)+len(b))
	for token := range a {
		union[token] = struct{}{}
		if _, ok := b[token]; ok {
			intersection++
		}
	}
	for token := range b {
		union[token] = struct{}{}
	}
	return float64(intersection) / float64(len(union))
}

func clusterKey(signals []model.Signal) string {
	counts := map[string]int{}
	for _, signal := range signals {
		for token := range tokenSet(signal.Title + " " + signal.Text) {
			counts[token]++
		}
	}
	type pair struct {
		word  string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for word, count := range counts {
		pairs = append(pairs, pair{word, count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].word < pairs[j].word
		}
		return pairs[i].count > pairs[j].count
	})
	if len(pairs) == 0 {
		return "unknown"
	}
	limit := 3
	if len(pairs) < limit {
		limit = len(pairs)
	}
	parts := make([]string, 0, limit)
	for _, item := range pairs[:limit] {
		parts = append(parts, item.word)
	}
	return strings.Join(parts, "-")
}
