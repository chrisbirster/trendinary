package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultBaseURL = "https://hacker-news.firebaseio.com/v0"

type Item struct {
	ID          int    `json:"id"`
	By          string `json:"by,omitempty"`
	Score       int    `json:"score,omitempty"`
	Time        int64  `json:"time,omitempty"`
	Title       string `json:"title,omitempty"`
	Type        string `json:"type,omitempty"`
	URL         string `json:"url,omitempty"`
	Descendants int    `json:"descendants,omitempty"`
}

type Client struct {
	http    *http.Client
	baseURL string
}

func NewClient(client *http.Client) *Client {
	return NewClientWithBaseURL(client, defaultBaseURL)
}

func NewClientWithBaseURL(client *http.Client, baseURL string) *Client {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &Client{http: client, baseURL: baseURL}
}

func (c *Client) Top(ctx context.Context, limit int) ([]Item, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	var ids []int
	if err := c.get(ctx, c.baseURL+"/topstories.json", &ids); err != nil {
		return nil, fmt.Errorf("top stories: %w", err)
	}
	if len(ids) < limit {
		limit = len(ids)
	}

	items := make([]Item, 0, limit)
	for _, id := range ids[:limit] {
		var item Item
		if err := c.get(ctx, fmt.Sprintf("%s/item/%d.json", c.baseURL, id), &item); err != nil {
			return nil, fmt.Errorf("item %d: %w", id, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) get(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	return json.NewDecoder(res.Body).Decode(target)
}
