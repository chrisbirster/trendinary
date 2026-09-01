package editorial

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Service struct {
	store   *Store
	sources map[string]DiscoverySource
	client  *http.Client
	now     func() time.Time
}

func NewService(store *Store, client *http.Client, sources ...DiscoverySource) (*Service, error) {
	if store == nil { return nil, fmt.Errorf("editorial store is required") }
	if client == nil { client = &http.Client{Timeout: 12*time.Second} }
	service := &Service{store:store,sources:map[string]DiscoverySource{},client:client,now:func()time.Time{return time.Now().UTC()}}
	for _, source := range sources { if source != nil { service.sources[source.Name()] = source } }
	if _, ok := service.sources["techurls"]; !ok { service.sources["techurls"] = NewTechURLs(client) }
	if err := store.EnsureSource(context.Background(), Source{ID:"techurls",Name:"TechURLs",Kind:"aggregator",URL:techURLsEndpoint,Enabled:true}); err != nil { return nil,err }
	return service,nil
}

func (s *Service) Store() *Store { return s.store }

func (s *Service) Ingest(ctx context.Context, name string) (IngestionRun,error) {
	name = strings.ToLower(strings.TrimSpace(name))
	sourceAdapter, ok := s.sources[name]
	if !ok { return IngestionRun{}, fmt.Errorf("unknown editorial source %q",name) }
	source, found, err := s.store.Source(ctx,name)
	if err != nil { return IngestionRun{},err }
	if !found { return IngestionRun{},fmt.Errorf("source %q is not configured",name) }
	if !source.Enabled { return IngestionRun{},fmt.Errorf("source %q is disabled",name) }

	started := s.now().UTC()
	run := IngestionRun{ID:NewID("ingest"),SourceID:name,StartedAt:started,Status:"running"}
	if err := s.store.StartRun(ctx,run.ID,name,started); err != nil { return IngestionRun{},err }

	result, fetchErr := sourceAdapter.Fetch(ctx)
	run.Metrics.SectionsSeen=result.SectionsSeen
	run.Metrics.ItemsSeen=result.ItemsSeen
	run.Metrics.Malformed=result.Malformed
	run.Metrics.Errors=len(result.Errors)
	if fetchErr != nil {
		run.Status="failed"; run.Error=fetchErr.Error(); completed:=s.now().UTC(); run.CompletedAt=&completed
		_ = s.store.FinishRun(ctx,run)
		return run,fetchErr
	}

	seen := map[string]struct{}{}
	for _, discovered := range result.Items {
		canonical, err := NormalizeURL(discovered.URL)
		if err != nil || strings.TrimSpace(discovered.Title)=="" {
			run.Metrics.Malformed++
			continue
		}
		if _, duplicate := seen[canonical]; duplicate { run.Metrics.Duplicates++; continue }
		seen[canonical]=struct{}{}
		now:=s.now().UTC()
		score,components,serendipity,why:=ScoreEditorial(discovered,canonical)
		publisher:=strings.TrimSpace(discovered.ExternalSourceName)
		if publisher=="" { publisher=PublisherDomain(canonical) }
		contentType:=discovered.ContentType; if contentType=="" { contentType=DetectContentType(canonical) }
		item:=ContentItem{ID:NewID("content"),CanonicalURL:canonical,OriginalURL:discovered.URL,Title:strings.TrimSpace(discovered.Title),Publisher:publisher,PublisherDomain:PublisherDomain(canonical),ContentType:contentType,DiscoveredAt:now,UpdatedAt:now,State:StateInbox,EditorialScore:score,ScoreComponents:components,Serendipity:serendipity,WhyInteresting:why,EnrichmentStatus:EnrichmentPending}
		inserted,updated,err:=s.store.UpsertContent(ctx,item,Discovery{DiscoverySourceID:name,ExternalSourceName:publisher,SourceAgeText:discovered.SourceAgeText,DiscoveredAt:now,MetadataJSON:defaultJSON(discovered.MetadataJSON)})
		if err != nil { run.Metrics.Errors++; result.Errors=append(result.Errors,err.Error()); continue }
		if inserted { run.Metrics.ItemsInserted++ } else if updated { run.Metrics.ItemsUpdated++ } else { run.Metrics.Duplicates++ }
	}

	completed:=s.now().UTC(); run.CompletedAt=&completed
	switch { case run.Metrics.Errors>0 && run.Metrics.ItemsInserted+run.Metrics.ItemsUpdated>0: run.Status="partial"; case run.Metrics.Errors>0: run.Status="failed"; default: run.Status="success" }
	if len(result.Errors)>0 { sort.Strings(result.Errors); run.Error=strings.Join(result.Errors,"; ") }
	if err := s.store.FinishRun(ctx,run); err != nil { return run,err }
	return run,nil
}

func (s *Service) Enrich(ctx context.Context, id string) (ContentItem,error) {
	item,ok,err:=s.store.Content(ctx,id); if err!=nil{return ContentItem{},err}; if !ok{return ContentItem{},fmt.Errorf("content item not found")}
	metadata,enrichErr:=FetchMetadata(ctx,s.client,item.OriginalURL)
	status:=EnrichmentComplete; errorText:=""
	if enrichErr!=nil { status=EnrichmentFailed; errorText=enrichErr.Error() } else if metadata.Description=="" && metadata.CanonicalURL=="" { status=EnrichmentPartial }
	canonical:=""
	if metadata.CanonicalURL!="" { if normalized,err:=NormalizeURL(metadata.CanonicalURL);err==nil { canonical=normalized } }
	published:=""; if !metadata.PublishedAt.IsZero(){published=metadata.PublishedAt.UTC().Format(time.RFC3339Nano)}
	if err:=s.store.UpdateEnrichment(ctx,id,canonical,metadata.Description,metadata.ImageURL,metadata.Author,published,status,errorText);err!=nil{return ContentItem{},err}
	updated,_,err:=s.store.Content(ctx,id); return updated,err
}

func defaultJSON(value string) string { if strings.TrimSpace(value)=="" { return "{}" }; return value }
