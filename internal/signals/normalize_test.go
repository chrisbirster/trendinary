package signals_test

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	"github.com/chrisbirster/trendinary/internal/signals"
)

func TestHackerNewsExternalLinkKeepsOriginalPublisher(t *testing.T) {
	values := signals.HackerNews([]hackernews.Item{{
		ID: 42, Title: "Audacity 4.0 released", URL: "https://www.wired.com/story/audacity-4/", By: "reader",
	}})
	if len(values) != 1 {
		t.Fatalf("signals = %+v", values)
	}
	got := values[0]
	if got.Source.Domain != "wired.com" || got.Source.Name != "wired.com" {
		t.Fatalf("publisher = %+v, want wired.com", got.Source)
	}
	if got.DiscoveryChannel != "hacker-news" {
		t.Fatalf("discovery channel = %q, want hacker-news", got.DiscoveryChannel)
	}
}

func TestHackerNewsSelfPostUsesHackerNewsAsPublisher(t *testing.T) {
	values := signals.HackerNews([]hackernews.Item{{ID: 43, Title: "Ask HN: Example", By: "reader"}})
	if len(values) != 1 {
		t.Fatalf("signals = %+v", values)
	}
	got := values[0]
	if got.Source.Domain != "news.ycombinator.com" || got.Source.Name != "Hacker News" {
		t.Fatalf("publisher = %+v, want Hacker News", got.Source)
	}
	if got.DiscoveryChannel != "hacker-news" {
		t.Fatalf("discovery channel = %q, want hacker-news", got.DiscoveryChannel)
	}
}
