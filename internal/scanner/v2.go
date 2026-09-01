package scanner

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/signals"
)

// RunWithSourcesV2 is the calibrated multi-source path. It adds conservative
// anti-flood controls, entity-aware semantic cluster merging, and a stronger
// candidate gate while preserving the existing scoring/history contract.
func (s *Scanner) RunWithSourcesV2(ctx context.Context, live *recent.Store, extra []DiscoverySource) (Result,error) {
	if s.history==nil||s.memory==nil{return Result{},fmt.Errorf("scanner dependencies are incomplete")}
	warnings:=[]string{};discovery:=[]model.Signal{};var hnErr error
	if s.hn!=nil{items,err:=s.hn.Top(ctx,s.config.HackerNewsLimit);if err!=nil{hnErr=err;warnings=append(warnings,fmt.Sprintf("hacker news discovery: %v",err))}else{discovery=append(discovery,signals.HackerNews(items)...)} }
	if live!=nil{stream:=live.Recent(time.Time{});if len(stream)>maxStreamingDiscoverySignals{stream=stream[len(stream)-maxStreamingDiscoverySignals:]};discovery=append(discovery,stream...)}
	for _,source:=range extra{if source==nil{continue};values,err:=source.Discover(ctx);if err!=nil{warnings=append(warnings,fmt.Sprintf("%s discovery: %v",source.Name(),err));continue};discovery=append(discovery,values...)}
	discovery=deduplicateSignals(discovery);filtered:=filterDiscoveryNoise(discovery);discovery=filtered.Signals;if filtered.Suppressed>0{warnings=append(warnings,fmt.Sprintf("noise controls suppressed %d repeated/flood signals",filtered.Suppressed))}
	if len(discovery)==0{if hnErr!=nil{return Result{},fmt.Errorf("no discovery signals available: %w",hnErr)};return Result{},fmt.Errorf("no discovery signals available")}
	clustered:=engine.ClusterSignalsV2(discovery,s.config.ClusterThreshold);candidates:=make([]engine.Cluster,0,len(clustered));for _,cluster:=range clustered{if candidateClusterV2(cluster){candidates=append(candidates,cluster)}};seedClusters:=strongestClusters(candidates,maxCandidateClusters)
	if len(seedClusters)==0{return Result{Signals:len(discovery),Warnings:append(warnings,"no clusters met detection-quality v2 candidate gate")},nil}
	enriched:=append([]engine.Cluster(nil),seedClusters...);hydrationLimit:=s.config.EnrichClusters;if hydrationLimit>len(enriched){hydrationLimit=len(enriched)};for i:=0;i<hydrationLimit;i++{if err:=s.hydrateBlueskyCandidate(ctx,&enriched[i]);err!=nil{warnings=append(warnings,fmt.Sprintf("bluesky hydrate %q: %v",enriched[i].Key,err))}}
	if s.bluesky!=nil{limit:=s.config.EnrichClusters;if limit>len(enriched){limit=len(enriched)};for i:=0;i<limit;i++{query:=clusterQuery(enriched[i]);if query==""{continue};response,err:=s.bluesky.Search(ctx,query,s.config.BlueskyLimit);if err!=nil{warnings=append(warnings,fmt.Sprintf("bluesky %q: %v",query,err));continue};enriched[i].Signals=deduplicateSignals(append(enriched[i].Signals,signals.Bluesky(response.Posts)...));s.enrichBlueskyProfiles(ctx,&enriched[i])}}
	allSignals:=[]model.Signal{};for i:=range enriched{enriched[i].Signals=deduplicateSignals(enriched[i].Signals);allSignals=append(allSignals,enriched[i].Signals...)};allSignals=deduplicateSignals(allSignals);if err:=s.history.RecordSignals(ctx,allSignals);err!=nil{return Result{},fmt.Errorf("persist signals: %w",err)}
	now:=s.now().UTC();trends:=make([]model.Trend,0,len(enriched));for _,cluster:=range enriched{entity,err:=s.history.ResolveEntity(ctx,clusterName(cluster),cluster.Key,engine.CanonicalTerms(cluster.Signals,8),now);if err!=nil{warnings=append(warnings,fmt.Sprintf("identity %s: %v",cluster.Key,err));continue};stable:=cluster;stable.Key=entity.ID;trend,snapshot,err:=s.scoreCluster(ctx,stable,now);if err!=nil{warnings=append(warnings,fmt.Sprintf("score %s: %v",entity.ID,err));continue};trend.ID=entity.ID;trend.Slug=entity.Slug;trend.Aliases=entity.Aliases;trend.Name=clusterName(cluster);if err:=s.decorateTrend(ctx,&trend,entity,cluster,now);err!=nil{warnings=append(warnings,fmt.Sprintf("decorate %s: %v",entity.ID,err))};if err:=s.history.RecordSnapshot(ctx,snapshot);err!=nil{warnings=append(warnings,fmt.Sprintf("snapshot %s: %v",entity.ID,err));continue};trends=append(trends,trend)}
	sort.SliceStable(trends,func(i,j int)bool{if trends[i].Score==trends[j].Score{return trends[i].Slug<trends[j].Slug};return trends[i].Score>trends[j].Score});if len(trends)>s.config.PublishedTrendLimit{trends=trends[:s.config.PublishedTrendLimit]};for i:=range trends{trends[i].Rank=i+1};s.memory.ReplaceTrends(trends)
	return Result{Signals:len(allSignals),Clusters:len(enriched),Trends:len(trends),Warnings:warnings},nil
}
