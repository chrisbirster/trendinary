package wikipedia

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)
func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDiscoverUsesYesterdayAndFiltersUtilityPages(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/2026/09/01") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatal("missing user-agent")
		}
		body := `{"items":[{"articles":[{"article":"Main_Page","views":999999,"rank":1},{"article":"Artificial_intelligence","views":123456,"rank":2},{"article":"Special:Search","views":50000,"rank":3}]}]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	source := New(client, 10)
	source.now = func() time.Time { return time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC) }

	values, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 {
		t.Fatalf("values = %+v", values)
	}
	got := values[0]
	if got.ID != "wikipedia:Artificial_intelligence" || got.Title != "Artificial intelligence" || got.Engagement.Score != 123456 {
		t.Fatalf("signal = %+v", got)
	}
	if got.PublishedAt != "2026-09-01T08:00:00Z" {
		t.Fatalf("published = %q", got.PublishedAt)
	}
}
