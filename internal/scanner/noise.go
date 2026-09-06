package scanner

import (
	"sort"
	"strings"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

type noiseResult struct{Signals []model.Signal;Suppressed int}

// filterDiscoveryNoise applies conservative source-flood and repeated-text
// controls before O(n²) clustering. It is intentionally explainable and avoids
// trying to infer intent or bot identity from weak evidence.
func filterDiscoveryNoise(values []model.Signal) noiseResult {
	if len(values)==0{return noiseResult{}}
	byAuthor:=map[string]int{};byDomain:=map[string]int{};fingerprints:=map[string]int{}
	out:=make([]model.Signal,0,len(values));suppressed:=0
	for _,signal:=range values{
		domain:=signal.Source.Domain;if domain==""{domain=signal.Source.Name}
		author:=signal.AuthorID;if author==""{author=signal.Author}
		authorKey:=domain+":"+author
		fingerprint:=strings.ToLower(strings.Join(strings.Fields(signal.Title+" "+signal.Text)," "))
		if len(fingerprint)>240{fingerprint=fingerprint[:240]}
		if author!=""&&byAuthor[authorKey]>=8{suppressed++;continue}
		if domain!=""&&byDomain[domain]>=60{suppressed++;continue}
		if fingerprint!=""&&fingerprints[domain+":"+fingerprint]>=3{suppressed++;continue}
		out=append(out,signal);if author!=""{byAuthor[authorKey]++};if domain!=""{byDomain[domain]++};if fingerprint!=""{fingerprints[domain+":"+fingerprint]++}
	}
	return noiseResult{Signals:out,Suppressed:suppressed}
}

// candidateClusterV2 deliberately shares the same admission policy as the
// legacy multi-source path. Production must not maintain a second, looser
// definition of what counts as enough independent evidence for a public trend.
func candidateClusterV2(cluster engine.Cluster) bool {
	return candidateCluster(cluster)
}

func strongestClusters(values []engine.Cluster, limit int) []engine.Cluster {
	out:=append([]engine.Cluster(nil),values...)
	sort.SliceStable(out,func(i,j int)bool{left,right:=candidateWeight(out[i]),candidateWeight(out[j]);if left==right{return out[i].Key<out[j].Key};return left>right})
	if limit>0&&len(out)>limit{out=out[:limit]};return out
}
