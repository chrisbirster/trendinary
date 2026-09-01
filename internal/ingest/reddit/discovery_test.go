package reddit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	redditingest "github.com/chrisbirster/trendinary/internal/ingest/reddit"
)

func TestDiscoverRequiresOAuthCredentials(t *testing.T) {
	discovery := redditingest.New(nil, "", "", "", 10)
	if _, err := discovery.Discover(context.Background()); err == nil {
		t.Fatal("expected missing OAuth credentials error")
	}
}

func TestDiscoverUsesOAuthAndNormalizesListing(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "client" || pass != "secret" {
			t.Fatalf("unexpected basic auth: %q %q ok=%v", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"abc","token_type":"bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "bearer abc" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"children":[{"data":{"id":"xyz","name":"t3_xyz","title":"A strange topic is exploding","selftext":"context","author":"alice","subreddit":"technology","permalink":"/r/technology/comments/xyz/topic/","score":500,"num_comments":120,"created_utc":1788253200}}]}}`))
	})

	discovery := redditingest.NewWithEndpoints(server.Client(), "client", "secret", "trendinary-test", 10, server.URL+"/token", server.URL+"/feed")
	values, err := discovery.Discover(context.Background())
	if err != nil { t.Fatal(err) }
	if len(values) != 1 { t.Fatalf("signals = %d, want 1", len(values)) }
	got := values[0]
	if got.ID != "reddit:t3_xyz" || got.Source.Domain != "reddit.com" || got.Engagement.Score != 500 || got.Engagement.Replies != 120 {
		t.Fatalf("unexpected normalized signal: %+v", got)
	}
	if got.URL != "https://www.reddit.com/r/technology/comments/xyz/topic/" {
		t.Fatalf("url = %q", got.URL)
	}
}
