package bluesky_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
)

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "AT Protocol" {
			t.Fatalf("q = %q", got)
		}
		if got := r.URL.Query().Get("sort"); got != "latest" {
			t.Fatalf("sort = %q", got)
		}
		fmt.Fprint(w, "{\"posts\":[{\"uri\":\"at://did:plc:test/app.bsky.feed.post/abc\",\"author\":{\"did\":\"did:plc:test\",\"handle\":\"example.test\"},\"record\":{\"text\":\"AT Protocol is moving\",\"createdAt\":\"2026-08-30T20:00:00Z\"},\"likeCount\":12,\"repostCount\":3}]}")
	}))
	defer server.Close()

	client := bluesky.NewClientWithEndpoint(server.Client(), server.URL)
	result, err := client.Search(context.Background(), "AT Protocol", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Posts) != 1 || result.Posts[0].Author.Handle != "example.test" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestHydratePostsLoadsCountersAndCachesProfile(t *testing.T) {
	var profileRequests atomic.Int32
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/xrpc/app.bsky.feed.getPosts", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query()["uris"]; len(got) != 1 || got[0] != "at://did:plc:test/app.bsky.feed.post/abc" {
			t.Fatalf("uris = %+v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"posts":[{"uri":"at://did:plc:test/app.bsky.feed.post/abc","author":{"did":"did:plc:test","handle":"alice.test","displayName":"Alice","avatar":"https://img.test/a.png"},"record":{"text":"moving fast","createdAt":"2026-09-01T09:00:00Z"},"replyCount":7,"repostCount":8,"likeCount":99,"quoteCount":3}]}`)
	})
	mux.HandleFunc("/xrpc/app.bsky.actor.getProfile", func(w http.ResponseWriter, r *http.Request) {
		profileRequests.Add(1)
		fmt.Fprint(w, `{"did":"did:plc:test","handle":"alice.test","displayName":"Alice"}`)
	})

	client := bluesky.NewClientWithEndpoint(server.Client(), server.URL+"/xrpc/app.bsky.feed.searchPosts")
	posts, err := client.HydratePosts(context.Background(), []string{"at://did:plc:test/app.bsky.feed.post/abc"})
	if err != nil { t.Fatal(err) }
	post := posts["at://did:plc:test/app.bsky.feed.post/abc"]
	if post.LikeCount != 99 || post.RepostCount != 8 || post.Author.Handle != "alice.test" {
		t.Fatalf("unexpected hydrated post: %+v", post)
	}
	profile, err := client.Profile(context.Background(), "did:plc:test")
	if err != nil { t.Fatal(err) }
	if profile.Handle != "alice.test" || profile.DisplayName != "Alice" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if profileRequests.Load() != 0 {
		t.Fatalf("profile cache miss after hydration, requests=%d", profileRequests.Load())
	}
}
