package bluesky

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultEndpoint = "https://api.bsky.app/xrpc/app.bsky.feed.searchPosts"

type Author struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName,omitempty"`
}

type Record struct {
	Text      string `json:"text,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type Post struct {
	URI        string `json:"uri"`
	CID        string `json:"cid,omitempty"`
	Author     Author `json:"author"`
	Record     Record `json:"record"`
	ReplyCount int    `json:"replyCount,omitempty"`
	RepostCount int   `json:"repostCount,omitempty"`
	LikeCount  int    `json:"likeCount,omitempty"`
	QuoteCount int    `json:"quoteCount,omitempty"`
}

type SearchResponse struct {
	Cursor    string `json:"cursor,omitempty"`
	HitsTotal *int   `json:"hitsTotal,omitempty"`
	Posts     []Post `json:"posts"`
}

type Client struct {
	http     *http.Client
	endpoint string
}

func NewClient(client *http.Client) *Client {
	return NewClientWithEndpoint(client, defaultEndpoint)
}

func NewClientWithEndpoint(client *http.Client, endpoint string) *Client {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &Client{http: client, endpoint: endpoint}
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return SearchResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "trendinary/0.1 (+https://trendinary.com)")

	res, err := c.http.Do(req)
	if err != nil {
		return SearchResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return SearchResponse{}, fmt.Errorf("unexpected status %d", res.StatusCode)
	}

	var payload SearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return SearchResponse{}, err
	}
	return payload, nil
}
