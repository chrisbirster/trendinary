package scanner

import (
	"sort"
	"strings"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

type noiseResult struct {
	Signals    []model.Signal
	Suppressed int
}

// filterDiscoveryNoise applies conservative source-flood and repeated-text
// controls before O(n²) clustering. It is intentionally explainable and avoids
// trying to infer intent or bot identity from weak evidence.
func filterDiscoveryNoise(values []model.Signal) noiseResult {
	if len(values) == 0 {
		return noiseResult{}
	}
	byAuthor := map[string]int{}
	byDomain := map[string]int{}
	fingerprints := map[string]int{}
	out := make([]model.Signal, 0, len(values))
	suppressed := 0
	for _, signal := range values {
		if isLowInformationSocial(signal) {
			suppressed++
			continue
		}
		domain := signal.Source.Domain
		if domain == "" {
			domain = signal.Source.Name
		}
		author := signal.AuthorID
		if author == "" {
			author = signal.Author
		}
		authorKey := domain + ":" + author
		fingerprint := strings.ToLower(strings.Join(strings.Fields(signal.Title+" "+signal.Text), " "))
		if len(fingerprint) > 240 {
			fingerprint = fingerprint[:240]
		}
		if author != "" && byAuthor[authorKey] >= 8 {
			suppressed++
			continue
		}
		if domain != "" && byDomain[domain] >= 60 {
			suppressed++
			continue
		}
		if fingerprint != "" && fingerprints[domain+":"+fingerprint] >= 3 {
			suppressed++
			continue
		}
		out = append(out, signal)
		if author != "" {
			byAuthor[authorKey]++
		}
		if domain != "" {
			byDomain[domain]++
		}
		if fingerprint != "" {
			fingerprints[domain+":"+fingerprint]++
		}
	}
	return noiseResult{Signals: out, Suppressed: suppressed}
}

// candidateClusterV2 remains the strict corroboration gate used for quality
// evaluation and confidence semantics. NewsData may corroborate a fresher
// observation but may not independently confirm a public trend.
func candidateClusterV2(cluster engine.Cluster) bool {
	hasQualifyingSignal := false
	hasFreshSignal := false
	for _, signal := range cluster.Signals {
		if contextOnlyCandidateSignal(signal) || isLowInformationSocial(signal) {
			continue
		}
		hasQualifyingSignal = true
		if !strings.HasPrefix(signal.ID, "newsdata:") {
			hasFreshSignal = true
		}
	}
	if hasQualifyingSignal && !hasFreshSignal {
		return false
	}
	return candidateCluster(cluster)
}

// chartCandidateCluster is deliberately broader than the confirmation gate.
// The Top 20 is a chart of attention, not a list of already-confirmed facts.
// Single-source items may chart as EMERGING, while context-only and low-value
// social observations still cannot manufacture an entry.
func chartCandidateCluster(cluster engine.Cluster) bool {
	qualifying := 0
	hasFreshSignal := false
	for _, signal := range cluster.Signals {
		if contextOnlyCandidateSignal(signal) || isLowInformationSocial(signal) {
			continue
		}
		if strings.TrimSpace(signal.Title) == "" && len(strings.Fields(signal.Text)) < 4 {
			continue
		}
		qualifying++
		if !strings.HasPrefix(signal.ID, "newsdata:") {
			hasFreshSignal = true
		}
	}
	return qualifying > 0 && hasFreshSignal
}

func strongestClusters(values []engine.Cluster, limit int) []engine.Cluster {
	out := append([]engine.Cluster(nil), values...)
	sort.SliceStable(out, func(i, j int) bool {
		left, right := top20CandidateWeight(out[i]), top20CandidateWeight(out[j])
		if left == right {
			return out[i].Key < out[j].Key
		}
		return left > right
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func top20CandidateWeight(cluster engine.Cluster) int {
	values := independentEvidenceSignals(cluster.Signals)
	provenance := provenanceSummary(values)
	engagement := 0
	for _, signal := range values {
		engagement += signal.Engagement.Score + signal.Engagement.Likes + 2*signal.Engagement.Reposts + signal.Engagement.Replies + 2*signal.Engagement.Quotes
	}
	return engagement + len(values)*10 + provenance.PublisherCount*45 + provenance.PlatformCount*65
}
