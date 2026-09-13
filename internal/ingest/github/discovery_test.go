package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	githubingest "github.com/chrisbirster/trendinary/internal/ingest/github"
)

func TestDiscoverNormalizesRepositoriesAndUsesBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q", got)
		}
		if !strings.Contains(r.URL.Query().Get("q"), "stars:>=20") {
			t.Fatalf("unexpected query: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":42,"full_name":"example/trend","html_url":"https://github.com/example/trend","description":"new trend engine","stargazers_count":75,"forks_count":4,"open_issues_count":3,"created_at":"2026-09-01T09:00:00Z","owner":{"login":"example"}},{"id":43,"full_name":"spam/too-small","html_url":"https://github.com/spam/too-small","description":"low signal burst","stargazers_count":5,"forks_count":0,"open_issues_count":0,"created_at":"2026-09-01T09:00:00Z","owner":{"login":"spam"}}]}`))
	}))
	defer server.Close()

	discovery := githubingest.NewWithEndpoint(server.Client(), server.URL, "test-token", 20)
	values, err := discovery.Discover(context.Background())
	if err != nil { t.Fatal(err) }
	if len(values) != 1 { t.Fatalf("signals = %d, want 1", len(values)) }
	got := values[0]
	if got.ID != "github:repo:42" || got.Source.Domain != "github.com" || got.Engagement.Score != 75 || got.Author != "example" {
		t.Fatalf("unexpected normalized signal: %+v", got)
	}
}
