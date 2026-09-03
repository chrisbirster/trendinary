package engine

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/chrisbirster/trendinary/internal/model"
)

// ClusterSignalsV2 performs deterministic event resolution directly across
// normalized signals. It accepts a strong lexical match immediately, then uses
// headline overlap, named-entity overlap, and publication proximity to recover
// the same event when publishers phrase the headline differently.
func ClusterSignalsV2(input []model.Signal, threshold float64) []Cluster {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.42
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
	for i := 0; i < len(input); i++ {
		for j := i + 1; j < len(input); j++ {
			if SameEvent(input[i], input[j], threshold) {
				union(i, j)
			}
		}
	}
	groups := map[int][]model.Signal{}
	for i, signal := range input {
		groups[find(i)] = append(groups[find(i)], signal)
	}
	out := make([]Cluster, 0, len(groups))
	for _, signals := range groups {
		out = append(out, Cluster{Key: clusterKey(signals), Signals: signals})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Signals) == len(out[j].Signals) {
			return out[i].Key < out[j].Key
		}
		return len(out[i].Signals) > len(out[j].Signals)
	})
	return out
}

// SameEvent is intentionally inspectable. Domain equality is not evidence: two
// articles from the same publisher can be unrelated and the same event can be
// reported by dozens of different publishers.
func SameEvent(a, b model.Signal, lexicalThreshold float64) bool {
	termsA, termsB := SignalTerms(a), SignalTerms(b)
	lexical := similarity(termsA, termsB)
	if lexical >= lexicalThreshold {
		return true
	}
	overlap := overlapCoefficient(termsA, termsB)
	entitiesA, entitiesB := EntityKeys(a), EntityKeys(b)
	entity := similarity(entitiesA, entitiesB)
	sharedEntity := hasSharedKey(entitiesA, entitiesB)
	proximity := timeProximity(a.PublishedAt, b.PublishedAt)

	score := lexical*.25 + overlap*.40 + entity*.25 + proximity*.10
	if sharedEntity && overlap >= .30 {
		score += .15
	}
	if score > 1 {
		score = 1
	}
	return score >= .52 && (overlap >= .30 || entity >= .34)
}

func hasSharedKey(a, b map[string]struct{}) bool {
	for key := range a {
		if _, ok := b[key]; ok {
			return true
		}
	}
	return false
}

func timeProximity(a, b string) float64 {
	left, lok := parseSignalTime(a)
	right, rok := parseSignalTime(b)
	if !lok || !rok {
		return 0
	}
	delta := left.Sub(right)
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

func parseSignalTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// EntityKeys extracts only semantic hints from the event text: hashtags and
// capitalized name phrases. Publisher domains and URL paths are provenance and
// are deliberately excluded from event similarity.
func EntityKeys(signal model.Signal) map[string]struct{} {
	out := map[string]struct{}{}
	words := strings.Fields(ClusteringText(signal))
	phrase := []string{}
	flush := func() {
		if len(phrase) > 0 {
			joined := strings.ToLower(strings.Join(phrase, " "))
			if len(joined) >= 3 {
				out["name:"+joined] = struct{}{}
			}
			phrase = phrase[:0]
		}
	}
	for _, raw := range words {
		trimmed := strings.Trim(raw, ".,:;!?()[]{}\"'`“”")
		if strings.HasPrefix(trimmed, "#") && len(trimmed) > 1 {
			out["tag:"+canonicalToken(trimmed)] = struct{}{}
		}
		if startsUpper(trimmed) && !allUpperNoise(trimmed) {
			phrase = append(phrase, trimmed)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func startsUpper(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) {
			return unicode.IsUpper(r)
		}
	}
	return false
}

func allUpperNoise(value string) bool {
	letters, upper := 0, 0
	for _, r := range value {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	return letters <= 1 || upper == letters && letters <= 3
}
