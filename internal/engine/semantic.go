package engine

import (
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/chrisbirster/trendinary/internal/model"
)

// ClusterSignalsV2 keeps the transparent lexical pass, then merges lexical
// clusters only when their entity/term signatures provide additional evidence.
// It is deliberately deterministic: no opaque embedding service decides what
// becomes a trend.
func ClusterSignalsV2(input []model.Signal, threshold float64) []Cluster {
	base := ClusterSignals(input, threshold)
	if len(base) < 2 { return base }
	parent:=make([]int,len(base));for i:=range parent{parent[i]=i}
	var find func(int)int;find=func(x int)int{if parent[x]!=x{parent[x]=find(parent[x])};return parent[x]}
	union:=func(a,b int){ra,rb:=find(a),find(b);if ra!=rb{parent[rb]=ra}}
	for i:=0;i<len(base);i++{for j:=i+1;j<len(base);j++{if clusterSemanticSimilarity(base[i],base[j])>=0.68{union(i,j)}}}
	groups:=map[int][]model.Signal{}
	for i,cluster:=range base{root:=find(i);groups[root]=append(groups[root],cluster.Signals...)}
	out:=make([]Cluster,0,len(groups));for _,signals:=range groups{out=append(out,Cluster{Key:clusterKey(signals),Signals:signals})}
	sort.SliceStable(out,func(i,j int)bool{if len(out[i].Signals)==len(out[j].Signals){return out[i].Key<out[j].Key};return len(out[i].Signals)>len(out[j].Signals)})
	return out
}

func clusterSemanticSimilarity(a,b Cluster)float64{
	termsA,entitiesA:=clusterSignatures(a.Signals);termsB,entitiesB:=clusterSignatures(b.Signals)
	lexical:=similarity(termsA,termsB);entity:=similarity(entitiesA,entitiesB)
	sharedEntity:=false;for key:=range entitiesA{if _,ok:=entitiesB[key];ok{sharedEntity=true;break}}
	score:=lexical*.65+entity*.35;if sharedEntity{score+=.15};if score>1{score=1};return score
}

func clusterSignatures(signals []model.Signal)(map[string]struct{},map[string]struct{}){
	terms:=map[string]struct{}{};entities:=map[string]struct{}{}
	for _,signal:=range signals{for key:=range SignalTerms(signal){terms[key]=struct{}{}};for key:=range EntityKeys(signal){entities[key]=struct{}{}}}
	return terms,entities
}

// EntityKeys extracts high-precision, explainable entity hints: hashtags,
// capitalized name phrases, linked domains, and meaningful URL path roots.
func EntityKeys(signal model.Signal)map[string]struct{}{
	out:=map[string]struct{}{}
	words:=strings.Fields(signal.Title)
	phrase:=[]string{}
	flush:=func(){if len(phrase)>0{joined:=strings.ToLower(strings.Join(phrase," "));if len(joined)>=3{out["name:"+joined]=struct{}{}};phrase=phrase[:0]}}
	for _,raw:=range words{
		trimmed:=strings.Trim(raw,".,:;!?()[]{}\"'`“”")
		if strings.HasPrefix(trimmed,"#")&&len(trimmed)>1{out["tag:"+canonicalToken(trimmed)]=struct{}{}}
		if startsUpper(trimmed)&&!allUpperNoise(trimmed){phrase=append(phrase,trimmed)}else{flush()}
	};flush()
	if parsed,err:=url.Parse(signal.URL);err==nil&&parsed.Hostname()!=""{host:=strings.ToLower(strings.TrimPrefix(parsed.Hostname(),"www."));out["domain:"+host]=struct{}{};segments:=strings.Split(strings.Trim(parsed.Path,"/"),"/");if len(segments)>0&&segments[0]!=""&&segments[0]!="watch"&&segments[0]!="item"{out["path:"+host+":"+strings.ToLower(segments[0])]=struct{}{}}}
	return out
}

func startsUpper(value string)bool{for _,r:=range value{if unicode.IsLetter(r){return unicode.IsUpper(r)}};return false}
func allUpperNoise(value string)bool{letters:=0;upper:=0;for _,r:=range value{if unicode.IsLetter(r){letters++;if unicode.IsUpper(r){upper++}}};return letters<=1||upper==letters&&letters<=3}
