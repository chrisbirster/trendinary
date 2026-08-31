package signals

import (
	"fmt"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	"github.com/chrisbirster/trendinary/internal/model"
)

var (
	hackerNewsSource = model.Source{Name: "Hacker News", Domain: "news.ycombinator.com", URL: "https://news.ycombinator.com/"}
	blueskySource    = model.Source{Name: "Bluesky", Domain: "bsky.app", URL: "https://bsky.app/"}
)

func HackerNews(items []hackernews.Item) []model.Signal {
	out := make([]model.Signal, 0, len(items))
	for _, item := range items {
		link := item.URL
		if link == "" {
			link = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID)
		}
		published := ""
		if item.Time > 0 {
			published = time.Unix(item.Time, 0).UTC().Format(time.RFC3339)
		}
		out = append(out, model.Signal{
			ID:          fmt.Sprintf("hn:%d", item.ID),
			Source:      hackerNewsSource,
			Title:       item.Title,
			URL:         link,
			Author:      item.By,
			PublishedAt: published,
			Engagement: model.Engagement{
				Score:   item.Score,
				Replies: item.Descendants,
			},
		})
	}
	return out
}

func Bluesky(posts []bluesky.Post) []model.Signal {
	out := make([]model.Signal, 0, len(posts))
	for _, post := range posts {
		rkey := post.URI
		if index := strings.LastIndex(post.URI, "/"); index >= 0 && index+1 < len(post.URI) {
			rkey = post.URI[index+1:]
		}
		link := ""
		if post.Author.Handle != "" && rkey != "" {
			link = fmt.Sprintf("https://bsky.app/profile/%s/post/%s", post.Author.Handle, rkey)
		}
		out = append(out, model.Signal{
			ID:          "bsky:" + post.URI,
			Source:      blueskySource,
			Text:        post.Record.Text,
			URL:         link,
			Author:      post.Author.Handle,
			PublishedAt: post.Record.CreatedAt,
			Engagement: model.Engagement{
				Replies: post.ReplyCount,
				Likes:   post.LikeCount,
				Reposts: post.RepostCount,
				Quotes:  post.QuoteCount,
			},
		})
	}
	return out
}
