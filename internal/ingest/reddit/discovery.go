package reddit

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

	"github.com/chrisbirster/trendinary/internal/model"
)

const (
	defaultTokenEndpoint = "https://www.reddit.com/api/v1/access_token"
	defaultFeedEndpoint  = "https://oauth.reddit.com/r/all/hot"
)

var redditSource = model.Source{Name: "Reddit", Domain: "reddit.com", URL: "https://www.reddit.com/"}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type listing struct {
	Data struct {
		Children []struct {
			Data post `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type post struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	SelfText    string  `json:"selftext"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	Permalink   string  `json:"permalink"`
	URL         string  `json:"url"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	CreatedUTC  float64 `json:"created_utc"`
}

type Discovery struct {
	http          *http.Client
	clientID      string
	clientSecret  string
	userAgent     string
	limit         int
	tokenEndpoint string
	feedEndpoint  string

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
	cachedAt    time.Time
	cached      []model.Signal
}

func New(client *http.Client, clientID, clientSecret, userAgent string, limit int) *Discovery {
	return NewWithEndpoints(client, clientID, clientSecret, userAgent, limit, defaultTokenEndpoint, defaultFeedEndpoint)
}

func NewWithEndpoints(client *http.Client, clientID, clientSecret, userAgent string, limit int, tokenEndpoint, feedEndpoint string) *Discovery {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "trendinary/0.1 (+https://trendinary.com)"
	}
	return &Discovery{
		http:          client,
		clientID:      strings.TrimSpace(clientID),
		clientSecret:  strings.TrimSpace(clientSecret),
		userAgent:     userAgent,
		limit:         limit,
		tokenEndpoint: tokenEndpoint,
		feedEndpoint:  feedEndpoint,
	}
}

func (d *Discovery) Name() string { return "Reddit" }

// Discover uses Reddit's OAuth Data API. Trendinary deliberately has no HTML
// scraping fallback: deployments must configure approved API credentials and
// comply with Reddit's current developer/commercial-use terms.
func (d *Discovery) Discover(ctx context.Context) ([]model.Signal, error) {
	if d.clientID == "" || d.clientSecret == "" {
		return nil, fmt.Errorf("reddit OAuth credentials are not configured")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if time.Since(d.cachedAt) < 2*time.Minute && len(d.cached) > 0 {
		return cloneSignals(d.cached), nil
	}
	if d.token == "" || time.Now().UTC().After(d.tokenExpiry) {
		if err := d.refreshToken(ctx); err != nil {
			return nil, err
		}
	}

	endpoint, err := url.Parse(d.feedEndpoint)
	if err != nil {
		return nil, err
	}
	params := endpoint.Query()
	params.Set("limit", strconv.Itoa(d.limit))
	params.Set("raw_json", "1")
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "bearer "+d.token)
	req.Header.Set("User-Agent", d.userAgent)
	req.Header.Set("Accept", "application/json")
	res, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit listing status %d", res.StatusCode)
	}
	var payload listing
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	values := make([]model.Signal, 0, len(payload.Data.Children))
	for _, child := range payload.Data.Children {
		post := child.Data
		id := post.Name
		if id == "" {
			id = post.ID
		}
		if id == "" || post.Title == "" {
			continue
		}
		link := post.Permalink
		if strings.HasPrefix(link, "/") {
			link = "https://www.reddit.com" + link
		}
		published := ""
		if post.CreatedUTC > 0 {
			published = time.Unix(int64(post.CreatedUTC), 0).UTC().Format(time.RFC3339)
		}
		text := strings.TrimSpace(post.SelfText)
		if post.Subreddit != "" {
			text = strings.TrimSpace(text + " subreddit:" + post.Subreddit)
		}
		values = append(values, model.Signal{
			ID:          "reddit:" + id,
			Source:      redditSource,
			Title:       post.Title,
			Text:        text,
			URL:         link,
			Author:      post.Author,
			AuthorID:    post.Author,
			PublishedAt: published,
			Engagement: model.Engagement{
				Score:   post.Score,
				Replies: post.NumComments,
			},
		})
	}
	d.cachedAt = time.Now().UTC()
	d.cached = cloneSignals(values)
	return values, nil
}

func (d *Discovery) refreshToken(ctx context.Context) error {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(d.clientID, d.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", d.userAgent)
	res, err := d.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("reddit OAuth status %d", res.StatusCode)
	}
	var payload tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return err
	}
	if payload.AccessToken == "" {
		return fmt.Errorf("reddit OAuth returned no access token")
	}
	d.token = payload.AccessToken
	ttl := time.Duration(payload.ExpiresIn) * time.Second
	if ttl <= time.Minute {
		ttl = time.Hour
	}
	d.tokenExpiry = time.Now().UTC().Add(ttl - time.Minute)
	return nil
}

func cloneSignals(values []model.Signal) []model.Signal {
	out := make([]model.Signal, len(values))
	copy(out, values)
	return out
}
