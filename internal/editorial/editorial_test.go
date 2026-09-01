package editorial_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chrisbirster/trendinary/internal/editorial"
	"github.com/chrisbirster/trendinary/internal/history"
)

func TestParseTechURLsFixture(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "techurls.html"))
	if err != nil { t.Fatal(err) }
	defer file.Close()
	result, err := editorial.ParseTechURLs(file)
	if err != nil { t.Fatal(err) }
	if result.SectionsSeen != 2 { t.Fatalf("sections = %d", result.SectionsSeen) }
	if len(result.Items) != 4 { t.Fatalf("items = %d: %+v", len(result.Items), result.Items) }
	if result.Items[0].ExternalSourceName != "Hacker News" || result.Items[0].SourceAgeText != "2h" { t.Fatalf("first item = %+v", result.Items[0]) }
	if result.Items[1].ContentType != editorial.ContentDiscussion { t.Fatalf("discussion type = %q", result.Items[1].ContentType) }
	if result.Items[3].ContentType != editorial.ContentRepository { t.Fatalf("repository type = %q", result.Items[3].ContentType) }
}

func TestParseTechURLsMalformedDoesNotAbort(t *testing.T) {
	html := `<html><body><h2>Wired</h2><div><span>1h</span><a href="javascript:alert(1)">bad</a></div><div><span>2h</span><a href="https://example.com/good">Good item</a></div></body></html>`
	result, err := editorial.ParseTechURLs(stringsReader(html))
	if err != nil { t.Fatal(err) }
	if len(result.Items) != 1 || result.Items[0].Title != "Good item" { t.Fatalf("result = %+v", result) }
	if result.Malformed != 1 { t.Fatalf("malformed = %d", result.Malformed) }
}

func TestNormalizeURLConservative(t *testing.T) {
	got, err := editorial.NormalizeURL("HTTPS://Example.COM/story?id=42&utm_source=x&gclid=y#comments")
	if err != nil { t.Fatal(err) }
	if got != "https://example.com/story?id=42" { t.Fatalf("got %q", got) }
}

type fakeSource struct{ items []editorial.DiscoveredItem }
func (f fakeSource) Name() string { return "fixture" }
func (f fakeSource) Fetch(context.Context) (editorial.FetchResult, error) { return editorial.FetchResult{Items:f.items,SectionsSeen:1,ItemsSeen:len(f.items)},nil }

func TestDuplicateIngestionStateAndNotes(t *testing.T) {
	historical, err := history.Open(filepath.Join(t.TempDir(), "test.db")); if err != nil { t.Fatal(err) }; defer historical.Close()
	store, err := editorial.NewStore(historical.DB()); if err != nil { t.Fatal(err) }
	if err := store.EnsureSource(context.Background(), editorial.Source{ID:"fixture",Name:"Fixture",Kind:"aggregator",URL:"https://fixture.test",Enabled:true}); err != nil { t.Fatal(err) }
	source := fakeSource{items:[]editorial.DiscoveredItem{{Title:"A useful thing",URL:"https://example.com/a?utm_medium=feed",ExternalSourceName:"Example",SourceAgeText:"2h"}}}
	service, err := editorial.NewService(store,nil,source); if err != nil { t.Fatal(err) }
	first, err := service.Ingest(context.Background(),"fixture"); if err != nil { t.Fatal(err) }
	second, err := service.Ingest(context.Background(),"fixture"); if err != nil { t.Fatal(err) }
	if first.Metrics.ItemsInserted != 1 { t.Fatalf("first metrics = %+v", first.Metrics) }
	if second.Metrics.Duplicates != 1 || second.Metrics.ItemsInserted != 0 { t.Fatalf("second metrics = %+v", second.Metrics) }
	items, err := store.ListContent(context.Background(),editorial.StateInbox,"",false,"score",timeZero(),10); if err != nil { t.Fatal(err) }
	if len(items)!=1 { t.Fatalf("items = %d",len(items)) }
	id:=items[0].ID
	if err:=store.SetState(context.Background(),id,editorial.StateQueued);err!=nil{t.Fatal(err)}
	if err:=store.SetState(context.Background(),id,editorial.StateConsumed);err!=nil{t.Fatal(err)}
	yes:=true
	note,err:=store.PutNote(context.Background(),editorial.Note{ContentItemID:id,Text:"This changed how I think about the problem.",WorthSharing:&yes});if err!=nil{t.Fatal(err)}
	if note.WorthSharing==nil||!*note.WorthSharing{t.Fatalf("note = %+v",note)}
	updated,ok,err:=store.Content(context.Background(),id);if err!=nil||!ok{t.Fatalf("content: %v %v",ok,err)}
	if updated.State!=editorial.StateConsumed||updated.ConsumedAt==nil||updated.Note==nil{t.Fatalf("updated = %+v",updated)}
	runs,err:=store.Runs(context.Background(),"fixture",10);if err!=nil{t.Fatal(err)}
	if len(runs)!=2||runs[0].Status!="success"{t.Fatalf("runs = %+v",runs)}
}

func stringsReader(value string) *strings.Reader { return strings.NewReader(value) }
func timeZero() time.Time { return time.Time{} }
