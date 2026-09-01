package bluesky

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseEndpoint = "https://public.api.bsky.app"
	defaultEndpoint     = defaultBaseEndpoint + "/xrpc/app.bsky.feed.searchPosts"
	profileCacheTTL     = 15 * time.Minute
)

type Author struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

type Record struct {
	Text      string `json:"text,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type Post struct {
	URI         string `json:"uri"`
	CID         string `json:"cid,omitempty"`
	Author      Author `json:"author"`
	Record      Record `json:"record"`
	ReplyCount  int    `json:"replyCount,omitempty"`
	RepostCount int    `json:"repostCount,omitempty"`
	LikeCount   int    `json:"likeCount,omitempty"`
	QuoteCount  int    `json:"quoteCount,omitempty"`
}

type SearchResponse struct {
	Cursor    string `json:"cursor,omitempty"`
	HitsTotal *int   `json:"hitsTotal,omitempty"`
	Posts     []Post `json:"posts"`
}

type PostsResponse struct {
	Posts []Post `json:"posts"`
}

type cachedProfile struct {
	profile   Author
	expiresAt time.Time
}

type Client struct {
	http       *http.Client
	endpoint   string
	base       string
	profileMu  sync.RWMutex
	profiles   map[string]cachedProfile
}

func NewClient(client *http.Client) *Client {
	return NewClientWithEndpoint(client, defaultEndpoint)
}

func NewClientWithEndpoint(client *http.Client, endpoint string) *Client {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	base := defaultBaseEndpoint
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		base = parsed.Scheme + "://" + parsed.Host
	}
	return &Client{
		http:     client,
		endpoint: endpoint,
		base:     base,
		profiles: make(map[string]cachedProfile),
	}
}

func (c *Client) Search(ctx context.Context, query string, limit int) (SearchResponse, error) {
	if query == "" {
		return SearchResponse{}, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	endpoint, err := url.Parse(c.endpoint)
	if err != nil {
		return SearchResponse{}, err
	}
	params := endpoint.Query()
	params.Set("q", query)
	params.Set("sort", "latest")
	params.Set("limit", strconv.Itoa(limit))
	endpoint.RawQuery = params.Encode()

	var payload SearchResponse
	if err := c.getJSON(ctx, endpoint.String(), &payload); err != nil {
		return SearchResponse{}, err
	}
	for _, post := range payload.Posts {
		c.rememberProfile(post.Author)
	}
	return payload, nil
}

// HydratePosts selectively loads AppView counters for posts that have already
// passed Trendinary's cheap candidate gate. Jetstream itself intentionally does
// not carry the continuously changing like/repost/reply counters.
func (c *Client) HydratePosts(ctx context.Context, uris []string) (map[string]Post, error) {
	uris = uniqueNonEmpty(uris, 25)
	if len(uris) == 0 {
		return map[string]Post{}, nil
	}
	endpoint, err := url.Parse(c.base + "/xrpc/app.bsky.feed.getPosts")
	if err != nil {
		return nil, err
	}
	params := endpoint.Query()
	for _, uri := range uris {
		params.Add("uris", uri)
	}
	endpoint.RawQuery = params.Encode()

	var payload PostsResponse
	if err := c.getJSON(ctx, endpoint.String(), &payload); err != nil {
		return nil, err
	}
	out := make(map[string]Post, len(payload.Posts))
	for _, post := range payload.Posts {
		out[post.URI] = post
		c.rememberProfile(post.Author)
	}
	return out, nil
}

// Profile resolves a DID or handle using the public Bluesky AppView and caches
// the result. Stable DIDs remain the identity key; handles are display metadata
// and may change over time.
func (c *Client) Profile(ctx context.Context, actor string) (Author, error) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return Author{}, fmt.Errorf("actor is required")
	}
	if cached, ok := c.cachedProfile(actor); ok {
		return cached, nil
	}
	endpoint, err := url.Parse(c.base + "/xrpc/app.bsky.actor.getProfile")
	if err != nil {
		return Author{}, err
	}
	params := endpoint.Query()
	params.Set("actor", actor)
	endpoint.RawQuery = params.Encode()

	var profile Author
	if err := c.getJSON(ctx, endpoint.String(), &profile); err != nil {
		return Author{}, err
	}
	c.rememberProfile(profile)
	return profile, nil
}

func (c *Client) Profiles(ctx context.Context, actors []string) map[string]Author {
	actors = uniqueNonEmpty(actors, 20)
	out := make(map[string]Author, len(actors))
	for _, actor := range actors {
		profile, err := c.Profile(ctx, actor)
		if err != nil {
			continue
		}
		out[actor] = profile
		if profile.DID != "" {
			out[profile.DID] = profile
		}
		if profile.Handle != "" {
			out[profile.Handle] = profile
		}
	}
	return out
}

func (c *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "trendinary/0.1 (+https://trendinary.com)")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func (c *Client) cachedProfile(actor string) (Author, bool) {
	c.profileMu.RLock()
	entry, ok := c.profiles[actor]
	c.profileMu.RUnlock()
	if !ok || time.Now().UTC().After(entry.expiresAt) {
		return Author{}, false
	}
	return entry.profile, true
}

func (c *Client) rememberProfile(profile Author) {
	if profile.DID == "" && profile.Handle == "" {
		return
	}
	entry := cachedProfile{profile: profile, expiresAt: time.Now().UTC().Add(profileCacheTTL)}
	c.profileMu.Lock()
	if profile.DID != "" {
		c.profiles[profile.DID] = entry
	}
	if profile.Handle != "" {
		c.profiles[profile.Handle] = entry
	}
	c.profileMu.Unlock()
}

func uniqueNonEmpty(values []string, limit int) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
