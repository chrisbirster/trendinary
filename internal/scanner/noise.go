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

func candidateClusterV2(cluster engine.Cluster) bool {
	if len(cluster.Signals)==0{return false}
	authors:=map[string]struct{}{};domains:=map[string]struct{}{};strongSingle:=false
	for _,signal:=range cluster.Signals{
		domain:=signal.Source.Domain;if domain==""{domain=signal.Source.Name};if domain!=""{domains[domain]=struct{}{}}
		identity:=signal.AuthorID;if identity==""{identity=signal.Author};if identity!=""{authors[domain+":"+identity]=struct{}{}}
		eng:=signal.Engagement.Score+signal.Engagement.Likes+signal.Engagement.Reposts+signal.Engagement.Replies+signal.Engagement.Quotes
		if eng>=20{strongSingle=true}
	}
	if len(cluster.Signals)>=2 && (len(domains)>=2 || len(authors)>=2){return true}
	if len(cluster.Signals)==1{return strongSingle}
	// Stream-only bursts need independent voices even if source breadth is one.
	if len(domains)==1{for domain:=range domains{if domain=="bsky.app"{return len(authors)>=minStreamOnlyAuthors}}}
	return strongSingle
}

func strongestClusters(values []engine.Cluster, limit int) []engine.Cluster {
	out:=append([]engine.Cluster(nil),values...)
	sort.SliceStable(out,func(i,j int)bool{left,right:=candidateWeight(out[i]),candidateWeight(out[j]);if left==right{return out[i].Key<out[j].Key};return left>right})
	if limit>0&&len(out)>limit{out=out[:limit]};return out
}
