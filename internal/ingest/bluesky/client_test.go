package bluesky_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
