package gdelt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

const defaultEndpoint = "https://api.gdeltproject.org/api/v2/doc/doc"

const defaultQuery = `(technology OR "artificial intelligence" OR cybersecurity OR startup OR science OR gaming)`

type article struct {
	URL           string `json:"url"`
	URLMobile     string `json:"url_mobile"`
	Title         string `json:"title"`
	SeenDate      string `json:"seendate"`
	SocialImage   string `json:"socialimage"`
	Domain        string `json:"domain"`
	Language      string `json:"language"`
	SourceCountry string `json:"sourcecountry"`
}

type response struct {
	Articles []article `json:"articles"`
}

type Source struct {
	client     *http.Client
	endpoint   string
	query      string
	maxRecords int
	timespan   string
	cacheTTL   time.Duration

	mu       sync.Mutex
	cachedAt time.Time
	cached   []model.Signal
}

func New(client *http.Client, query string, maxRecords int) *Source {
	return NewWithEndpoint(client, defaultEndpoint, query, maxRecords)
}

func NewWithEndpoint(client *http.Client, endpoint, query string, maxRecords int) *Source {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(query) == "" {
		query = defaultQuery
	}
	if maxRecords <= 0 {
		maxRecords = 250
	}
	if maxRecords > 250 {
		maxRecords = 250
	}
	return &Source{
		client:     client,
		endpoint:   endpoint,
		query:      query,
		maxRecords: maxRecords,
		timespan:   "1h",
		cacheTTL:   10 * time.Minute,
	}
}

func (s *Source) Name() string { return "GDELT" }

func (s *Source) Discover(ctx context.Context) ([]model.Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cachedAt.IsZero() && time.Since(s.cachedAt) < s.cacheTTL {
		return cloneSignals(s.cached), nil
	}

	endpoint, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, err
	}
	params := endpoint.Query()
	params.Set("query", s.query)
	params.Set("mode", "artlist")
	params.Set("format", "json")
	params.Set("maxrecords", strconv.Itoa(s.maxRecords))
	params.Set("timespan", s.timespan)
	params.Set("sort", "datedesc")
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Trendinary/0.3.1 (+https://trendinary.com; public-discovery)")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("gdelt returned HTTP %d", res.StatusCode)
	}

	var payload response
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode gdelt: %w", err)
	}

	values := make([]model.Signal, 0, len(payload.Articles))
	seen := make(map[string]struct{}, len(payload.Articles))
	for _, item := range payload.Articles {
		link := strings.TrimSpace(item.URL)
		title := strings.TrimSpace(item.Title)
		if link == "" || title == "" {
			continue
		}
		if _, exists := seen[link]; exists {
			continue
		}
		seen[link] = struct{}{}

		domain := strings.TrimSpace(strings.ToLower(item.Domain))
		if domain == "" {
			domain = hostname(link)
		}
		name := domain
		if name == "" {
			name = "GDELT publisher"
		}
		sum := sha256.Sum256([]byte(link))
		values = append(values, model.Signal{
			ID:               "gdelt:" + hex.EncodeToString(sum[:8]),
			Source:           model.Source{Name: name, Domain: domain, URL: link},
			DiscoveryChannel: "gdelt",
			Title:            title,
			ClusterText:      title,
			URL:              link,
			PublishedAt:      parseSeenDate(item.SeenDate),
		})
	}

	s.cachedAt = time.Now().UTC()
	s.cached = cloneSignals(values)
	return values, nil
}

func parseSeenDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{"20060102T150405Z", "20060102150405", time.RFC3339, time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func hostname(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
}

func cloneSignals(values []model.Signal) []model.Signal {
	out := make([]model.Signal, len(values))
	copy(out, values)
	return out
}
