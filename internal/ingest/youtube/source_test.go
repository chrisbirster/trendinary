package youtube

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)
func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDiscoverMostPopularTechnologyVideos(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		query := r.URL.Query()
		if query.Get("part") != "snippet,statistics" || query.Get("chart") != "mostPopular" || query.Get("regionCode") != "US" || query.Get("videoCategoryId") != "28" || query.Get("maxResults") != "10" || query.Get("key") != "secret" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatal("missing user-agent")
		}
		body := `{"items":[{"id":"abc123","snippet":{"publishedAt":"2026-09-01T12:00:00Z","channelTitle":"Example Channel","title":"New computing hardware","description":"A detailed look."},"statistics":{"viewCount":"100000","likeCount":"5000","commentCount":"700"}}]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}

	values, err := New(client, "secret", "US", 10).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 {
		t.Fatalf("values = %+v", values)
	}
	got := values[0]
	if got.ID != "youtube:abc123" || got.Source.Domain != "youtube.com" || got.Author != "Example Channel" {
		t.Fatalf("signal = %+v", got)
	}
	if got.Engagement.Score != 100000 || got.Engagement.Likes != 5000 || got.Engagement.Replies != 700 {
		t.Fatalf("engagement = %+v", got.Engagement)
	}
}

func TestDiscoverRequiresAPIKey(t *testing.T) {
	if _, err := New(nil, "", "US", 10).Discover(context.Background()); err == nil {
		t.Fatal("expected missing API key error")
	}
}
