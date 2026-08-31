package hackernews_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
)

func TestTop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/topstories.json", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[101,102]`)
	})
	mux.HandleFunc("/item/101.json", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":101,"title":"First signal","score":42,"type":"story"}`)
	})
	mux.HandleFunc("/item/102.json", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":102,"title":"Second signal","score":21,"type":"story"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := hackernews.NewClientWithBaseURL(server.Client(), server.URL)
	items, err := client.Top(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != 101 || items[1].ID != 102 {
		t.Fatalf("unexpected items: %+v", items)
	}
}
