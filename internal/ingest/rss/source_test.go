package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverRSSAndAtomFeeds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rss", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Fatal("RSS request missing user-agent")
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel><title>Example News</title><item><title>RSS Story</title><link>https://publisher.example/rss-story</link><description>Useful summary</description><pubDate>Tue, 01 Sep 2026 12:00:00 +0000</pubDate><author>Alice</author></item></channel></rss>`))
	})
	mux.HandleFunc("/atom", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>Example Atom</title><entry><title>Atom Story</title><summary>Atom summary</summary><published>2026-09-01T13:00:00Z</published><author><name>Bob</name></author><link rel="alternate" href="https://atom.example/story"/></entry></feed>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	values, err := New(server.Client(), []string{server.URL + "/rss", server.URL + "/atom"}, 10).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 {
		t.Fatalf("values = %+v", values)
	}
	if values[0].Source.Name != "Example News" || values[0].Source.Domain != "publisher.example" || values[0].Author != "Alice" || values[0].PublishedAt != "2026-09-01T12:00:00Z" {
		t.Fatalf("rss = %+v", values[0])
	}
	if values[1].Source.Name != "Example Atom" || values[1].URL != "https://atom.example/story" || values[1].Author != "Bob" {
		t.Fatalf("atom = %+v", values[1])
	}
}

func TestDiscoverSurvivesOneBrokenFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			http.Error(w, "nope", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel><title>Good</title><item><title>Story</title><link>https://good.example/story</link></item></channel></rss>`))
	}))
	defer server.Close()

	values, err := New(server.Client(), []string{server.URL + "/bad", server.URL + "/good"}, 10).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Title != "Story" {
		t.Fatalf("values = %+v", values)
	}
}

func TestConditionalRSSRequestReturnsCachedBatchAndReports304(t *testing.T) {
	const etag = `"v1"`
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 1 {
			if r.Header.Get("If-None-Match") != etag {
				t.Fatalf("If-None-Match = %q, want %q", r.Header.Get("If-None-Match"), etag)
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel><title>Conditional</title><item><title>Cached Story</title><link>https://cached.example/story</link></item></channel></rss>`))
	}))
	defer server.Close()

	source := NewFeed(server.Client(), "Conditional", server.URL, 10)
	first, err := source.Discover(context.Background())
	if err != nil || len(first) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := source.Discover(context.Background())
	if err != nil || len(second) != 1 || second[0].ID != first[0].ID {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	requestCount, notModified := source.RequestDiagnostics()
	if requestCount != 2 || notModified != 1 {
		t.Fatalf("diagnostics requests=%d not_modified=%d", requestCount, notModified)
	}
}
