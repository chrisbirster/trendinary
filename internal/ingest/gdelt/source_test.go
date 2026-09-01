package gdelt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverNormalizesArticlesAndCaches(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		query := r.URL.Query()
		if query.Get("mode") != "artlist" || query.Get("format") != "json" || query.Get("maxrecords") != "2" || query.Get("timespan") != "1h" || query.Get("sort") != "datedesc" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatal("missing user-agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"articles":[{"url":"https://Example.COM/story","title":"A technology story","seendate":"20260901T120000Z","domain":"example.com"},{"url":"https://example.net/other","title":"Another story","seendate":"20260901T121500Z","domain":"example.net"}]}`))
	}))
	defer server.Close()

	source := NewWithEndpoint(server.Client(), server.URL, "technology", 2)
	first, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("signals = %d / %d", len(first), len(second))
	}
	if first[0].ID == "" || first[0].Source.Domain != "example.com" || first[0].PublishedAt != "2026-09-01T12:00:00Z" {
		t.Fatalf("first = %+v", first[0])
	}
}

func TestDiscoverSkipsMalformedAndDuplicateArticles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"articles":[{"url":"https://example.com/a","title":"A"},{"url":"https://example.com/a","title":"A again"},{"url":"","title":"bad"},{"url":"https://example.com/no-title","title":""}]}`))
	}))
	defer server.Close()

	values, err := NewWithEndpoint(server.Client(), server.URL, "technology", 10).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 {
		t.Fatalf("values = %+v", values)
	}
}
