package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type feedCache struct {
	ETag         string
	LastModified string
	Signals      []model.Signal
}

type Source struct {
	client *http.Client
	feeds  []string
	limit  int
	name   string

	mu    sync.Mutex
	cache map[string]feedCache
}

func New(client *http.Client, feeds []string, limit int) *Source {
	return newSource(client, "rss/news", feeds, limit)
}

// NewFeed creates a separately schedulable RSS adapter. Keeping each publisher
// feed distinct lets source operations show cadence, failures, and last success
// without turning RSS into one opaque mega-source.
func NewFeed(client *http.Client, name, feed string, limit int) *Source {
	if strings.TrimSpace(name) == "" {
		name = "rss:" + host(feed)
	}
	return newSource(client, name, []string{feed}, limit)
}

func newSource(client *http.Client, name string, feeds []string, limit int) *Source {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if limit <= 0 {
		limit = 40
	}
	return &Source{client: client, feeds: feeds, limit: limit, name: name, cache: map[string]feedCache{}}
}

func (s *Source) Name() string { return s.name }

func (s *Source) Discover(ctx context.Context) ([]model.Signal, error) {
	out := []model.Signal{}
	errs := []string{}
	for _, feed := range s.feeds {
		feed = strings.TrimSpace(feed)
		if feed == "" {
			continue
		}
		items, err := s.fetch(ctx, feed)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		out = append(out, items...)
		if len(out) >= s.limit {
			out = out[:s.limit]
			break
		}
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("rss discovery: %s", strings.Join(errs, "; "))
	}
	return out, nil
}

type rssDoc struct {
	Channel struct {
		Title string `xml:"title"`
		Link  string `xml:"link"`
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
			Author      string `xml:"author"`
		} `xml:"item"`
	} `xml:"channel"`
}

type atomDoc struct {
	Title   string `xml:"title"`
	Entries []struct {
		Title     string `xml:"title"`
		Summary   string `xml:"summary"`
		Updated   string `xml:"updated"`
		Published string `xml:"published"`
		Author    struct {
			Name string `xml:"name"`
		} `xml:"author"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

func (s *Source) fetch(ctx context.Context, feed string) ([]model.Signal, error) {
	s.mu.Lock()
	cached := s.cache[feed]
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Trendinary/0.3.1 (+https://trendinary.com; public-discovery)")
	if cached.ETag != "" {
		req.Header.Set("If-None-Match", cached.ETag)
	}
	if cached.LastModified != "" {
		req.Header.Set("If-Modified-Since", cached.LastModified)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return cloneSignals(cached.Signals), err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return cloneSignals(cached.Signals), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return cloneSignals(cached.Signals), fmt.Errorf("%s returned HTTP %d", feed, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return cloneSignals(cached.Signals), err
	}

	values, err := parseFeed(feed, data)
	if err != nil {
		return cloneSignals(cached.Signals), err
	}
	next := feedCache{
		ETag:         strings.TrimSpace(resp.Header.Get("ETag")),
		LastModified: strings.TrimSpace(resp.Header.Get("Last-Modified")),
		Signals:      cloneSignals(values),
	}
	s.mu.Lock()
	s.cache[feed] = next
	s.mu.Unlock()
	return values, nil
}

func parseFeed(feed string, data []byte) ([]model.Signal, error) {
	var rss rssDoc
	if xml.Unmarshal(data, &rss) == nil && len(rss.Channel.Items) > 0 {
		sourceName := strings.TrimSpace(rss.Channel.Title)
		if sourceName == "" {
			sourceName = host(feed)
		}
		out := make([]model.Signal, 0, len(rss.Channel.Items))
		for _, item := range rss.Channel.Items {
			link := strings.TrimSpace(item.Link)
			if link == "" {
				continue
			}
			out = append(out, signal(sourceName, link, item.Title, item.Description, item.Author, parseTime(item.PubDate)))
		}
		return out, nil
	}

	var atom atomDoc
	if err := xml.Unmarshal(data, &atom); err != nil {
		return nil, err
	}
	sourceName := strings.TrimSpace(atom.Title)
	if sourceName == "" {
		sourceName = host(feed)
	}
	out := make([]model.Signal, 0, len(atom.Entries))
	for _, entry := range atom.Entries {
		link := ""
		for _, candidate := range entry.Links {
			if candidate.Rel == "" || candidate.Rel == "alternate" {
				link = strings.TrimSpace(candidate.Href)
				if link != "" {
					break
				}
			}
		}
		if link == "" {
			continue
		}
		published := entry.Published
		if published == "" {
			published = entry.Updated
		}
		out = append(out, signal(sourceName, link, entry.Title, entry.Summary, entry.Author.Name, parseTime(published)))
	}
	return out, nil
}

func signal(sourceName, link, title, text, author, published string) model.Signal {
	sum := sha256.Sum256([]byte(link))
	title = strings.TrimSpace(title)
	return model.Signal{
		ID:               "rss:" + hex.EncodeToString(sum[:8]),
		Source:           model.Source{Name: sourceName, Domain: host(link), URL: link},
		DiscoveryChannel: "rss",
		Title:            title,
		ClusterText:      title,
		Text:             strip(text),
		URL:              link,
		Author:           strings.TrimSpace(author),
		PublishedAt:      published,
	}
}

func host(raw string) string {
	u, _ := url.Parse(raw)
	return strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
}

func parseTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, time.RFC3339, time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func strip(value string) string {
	value = strings.ReplaceAll(value, "<![CDATA[", "")
	value = strings.ReplaceAll(value, "]]>", "")
	if len(value) > 500 {
		value = value[:500]
	}
	return strings.TrimSpace(value)
}

func cloneSignals(values []model.Signal) []model.Signal {
	out := make([]model.Signal, len(values))
	copy(out, values)
	return out
}
