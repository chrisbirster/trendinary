package github

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

const defaultEndpoint = "https://api.github.com/search/repositories"

var githubSource = model.Source{Name: "GitHub", Domain: "github.com", URL: "https://github.com/"}

type repositoryOwner struct {
	Login string `json:"login"`
}

type repository struct {
	ID              int64           `json:"id"`
	FullName        string          `json:"full_name"`
	HTMLURL         string          `json:"html_url"`
	Description     string          `json:"description"`
	StargazersCount int             `json:"stargazers_count"`
	ForksCount      int             `json:"forks_count"`
	OpenIssuesCount int             `json:"open_issues_count"`
	CreatedAt       string          `json:"created_at"`
	PushedAt        string          `json:"pushed_at"`
	Owner           repositoryOwner `json:"owner"`
}

type searchResponse struct {
	Items []repository `json:"items"`
}

type Discovery struct {
	http     *http.Client
	endpoint string
	token    string
	limit    int
	mu       sync.Mutex
	cachedAt time.Time
	cached   []model.Signal
}

func New(client *http.Client, token string, limit int) *Discovery {
	return NewWithEndpoint(client, defaultEndpoint, token, limit)
}

func NewWithEndpoint(client *http.Client, endpoint, token string, limit int) *Discovery {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if limit <= 0 {
		limit = 40
	}
	if limit > 100 {
		limit = 100
	}
	return &Discovery{http: client, endpoint: endpoint, token: strings.TrimSpace(token), limit: limit}
}

func (d *Discovery) Name() string { return "GitHub" }

// Discover finds newly-created repositories attracting unusual early stars.
// The five-minute cache keeps unauthenticated installations comfortably below
// GitHub's public rate limit; a token is strongly preferred in production.
func (d *Discovery) Discover(ctx context.Context) ([]model.Signal, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if time.Since(d.cachedAt) < 5*time.Minute && len(d.cached) > 0 {
		return cloneSignals(d.cached), nil
	}

	endpoint, err := url.Parse(d.endpoint)
	if err != nil {
		return nil, err
	}
	params := endpoint.Query()
	params.Set("q", "created:>="+time.Now().UTC().Add(-48*time.Hour).Format("2006-01-02")+" stars:>=5")
	params.Set("sort", "stars")
	params.Set("order", "desc")
	params.Set("per_page", strconv.Itoa(d.limit))
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "trendinary/0.3.1 (+https://trendinary.com)")
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}
	res, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github search status %d", res.StatusCode)
	}
	var payload searchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	values := make([]model.Signal, 0, len(payload.Items))
	for _, repo := range payload.Items {
		if repo.ID == 0 || repo.FullName == "" {
			continue
		}
		values = append(values, model.Signal{
			ID:               fmt.Sprintf("github:repo:%d", repo.ID),
			Source:           githubSource,
			DiscoveryChannel: "github",
			Title:            repo.FullName,
			ClusterText:      repo.FullName + " " + repo.Description,
			Text:             repo.Description,
			URL:              repo.HTMLURL,
			Author:           repo.Owner.Login,
			AuthorID:         repo.Owner.Login,
			PublishedAt:      repo.CreatedAt,
			Engagement: model.Engagement{
				Score:   repo.StargazersCount,
				Replies: repo.OpenIssuesCount,
				Reposts: repo.ForksCount,
			},
		})
	}
	d.cachedAt = time.Now().UTC()
	d.cached = cloneSignals(values)
	return values, nil
}

func cloneSignals(values []model.Signal) []model.Signal {
	out := make([]model.Signal, len(values))
	copy(out, values)
	return out
}
