package engine

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/chrisbirster/trendinary/internal/model"
)

var genericEntityTokens = map[string]struct{}{
	"ask": {}, "breaking": {}, "how": {}, "list": {}, "new": {}, "show": {},
	"the": {}, "this": {}, "today": {}, "what": {}, "when": {}, "where": {}, "why": {},
	"more": {}, "less": {}, "yes": {}, "no": {}, "now": {}, "yeah": {}, "yep": {},
	"nope": {}, "okay": {}, "ok": {}, "same": {}, "true": {}, "really": {}, "maybe": {},
}

// ClusterSignalsV2 performs deterministic event resolution directly across
// normalized signals. A signal may join an existing cluster only when it
// matches every signal already in that cluster. This complete-link rule is
// deliberately conservative: it prevents bridge observations (A≈B and B≈C)
// from collapsing unrelated A and C events into one public trend.
func ClusterSignalsV2(input []model.Signal, threshold float64) []Cluster {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.42
	}
	values := append([]model.Signal(nil), input...)
	sort.SliceStable(values, func(i, j int) bool {
		left, right := signalSortKey(values[i]), signalSortKey(values[j])
		return left < right
	})

	clusters := make([]Cluster, 0, len(values))
	for _, signal := range values {
		placed := false
		for index := range clusters {
			if matchesEntireCluster(signal, clusters[index], threshold) {
				clusters[index].Signals = append(clusters[index].Signals, signal)
				placed = true
				break
			}
		}
		if !placed {
			clusters = append(clusters, Cluster{Signals: []model.Signal{signal}})
		}
	}
	for index := range clusters {
		clusters[index].Key = clusterKey(clusters[index].Signals)
	}
	sort.SliceStable(clusters, func(i, j int) bool {
		if len(clusters[i].Signals) == len(clusters[j].Signals) {
			return clusters[i].Key < clusters[j].Key
		}
		return len(clusters[i].Signals) > len(clusters[j].Signals)
	})
	return clusters
}

func signalSortKey(signal model.Signal) string {
	return strings.Join([]string{
		strings.TrimSpace(signal.PublishedAt),
		strings.TrimSpace(signal.ID),
		strings.ToLower(strings.TrimSpace(ClusteringText(signal))),
	}, "\x00")
}

func matchesEntireCluster(signal model.Signal, cluster Cluster, threshold float64) bool {
	if len(cluster.Signals) == 0 {
		return true
	}
	for _, member := range cluster.Signals {
		if !SameEvent(signal, member, threshold) {
			return false
		}
	}
	return true
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
	// Different publishers often share only the central named entity while
	// choosing completely different verbs and modifiers. A 20% term overlap is
	// enough additional evidence when that entity agrees and timing is close;
	// the final score still rejects unrelated stories about the same company.
	if sharedEntity && overlap >= .20 {
		score += .15
	}
	if score > 1 {
		score = 1
	}
	return score >= .52 && (overlap >= .20 || entity >= .34)
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
			if len(phrase) == 1 {
				if _, generic := genericEntityTokens[joined]; generic {
					phrase = phrase[:0]
					return
				}
			}
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
