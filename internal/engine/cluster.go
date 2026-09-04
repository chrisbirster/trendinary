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
	"that": {}, "the": {}, "this": {}, "to": {}, "with": {}, "new": {}, "latest": {},
}

// phraseAliases only contains spelling/name variants that refer to the same
// entity. Related-but-distinct entities (for example OpenAI and ChatGPT) must
// not be collapsed here merely because they often appear together.
var phraseAliases = []struct {
	canonical string
	variants  []string
}{
	{canonical: "atproto", variants: []string{"at protocol", "at-protocol", "at proto", "atproto"}},
	{canonical: "solidjs", variants: []string{"solid.js", "solid-js", "solid js", "solidjs"}},
	{canonical: "javascript", variants: []string{"java script", "javascript"}},
	{canonical: "typescript", variants: []string{"type script", "typescript"}},
	{canonical: "hackernews", variants: []string{"hacker news", "hackernews"}},
	{canonical: "github", variants: []string{"git hub", "github"}},
}

// ClusterSignals is the transparent lexical baseline. Only semantic event text
// participates in similarity. Publisher domains and adapter metadata are
// provenance and must not make unrelated stories look similar.
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
		tokens[i] = SignalTerms(signal)
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

// ClusteringText returns the event-bearing text for a signal. Adapters can set
// ClusterText when Text contains descriptive boilerplate that is useful for the
// UI but dangerous for similarity. Headlines are otherwise preferred; social
// observations naturally fall back to their body text.
func ClusteringText(signal model.Signal) string {
	if value := strings.TrimSpace(signal.ClusterText); value != "" {
		return value
	}
	if value := strings.TrimSpace(signal.Title); value != "" {
		return value
	}
	return strings.TrimSpace(signal.Text)
}

// SignalTerms returns canonical, meaningful event terms used by clustering and
// stable trend identity. Source hostnames are intentionally excluded.
func SignalTerms(signal model.Signal) map[string]struct{} {
	return tokenSet(canonicalize(ClusteringText(signal)))
}

// CanonicalTerms returns the most representative canonical terms across a set
// of signals. Stable trend identity uses these terms to reconnect a changing
// wording cluster to an existing long-lived Trendinary entity.
func CanonicalTerms(signals []model.Signal, limit int) []string {
	if limit <= 0 {
		limit = 8
	}
	counts := map[string]int{}
	for _, signal := range signals {
		for token := range SignalTerms(signal) {
			counts[token]++
		}
	}
	type pair struct {
		word  string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for word, count := range counts {
		if word == "" {
			continue
		}
		pairs = append(pairs, pair{word: word, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].word < pairs[j].word
		}
		return pairs[i].count > pairs[j].count
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]string, 0, len(pairs))
	for _, item := range pairs {
		out = append(out, item.word)
	}
	return out
}

func canonicalize(text string) string {
	text = strings.ToLower(text)
	// Longest phrases first prevents a shorter variant from partially consuming
	// a longer one. Word-like padding keeps replacements from joining neighbors.
	for _, group := range phraseAliases {
		variants := append([]string(nil), group.variants...)
		sort.SliceStable(variants, func(i, j int) bool { return len(variants[i]) > len(variants[j]) })
		for _, variant := range variants {
			text = strings.ReplaceAll(text, variant, " "+group.canonical+" ")
		}
	}
	return text
}

func canonicalToken(token string) string {
	token = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(token, "#")))
	for _, group := range phraseAliases {
		for _, variant := range group.variants {
			compact := strings.NewReplacer(" ", "", "-", "", ".", "").Replace(variant)
			if strings.NewReplacer(" ", "", "-", "", ".", "").Replace(token) == compact {
				return group.canonical
			}
		}
	}
	return token
}

func tokenSet(text string) map[string]struct{} {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '#'
	})
	set := make(map[string]struct{}, len(words))
	for _, word := range words {
		word = canonicalToken(word)
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

func overlapCoefficient(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for token := range a {
		if _, ok := b[token]; ok {
			intersection++
		}
	}
	denominator := len(a)
	if len(b) < denominator {
		denominator = len(b)
	}
	return float64(intersection) / float64(denominator)
}

func clusterKey(signals []model.Signal) string {
	terms := CanonicalTerms(signals, 3)
	if len(terms) == 0 {
		return "unknown"
	}
	return strings.Join(terms, "-")
}
