package newsdata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
)

const defaultEndpoint = "https://newsdata.io/api/1/latest"

type quotaStore interface {
	ReserveDailyQuota(context.Context, string, int, time.Time) (history.DailyQuotaStatus, bool, error)
	DailyQuota(context.Context, string, int, time.Time) (history.DailyQuotaStatus, error)
}

type signalRecorder interface {
	RecordSignals(context.Context, []model.Signal) error
}

type article struct {
	ArticleID   string   `json:"article_id"`
	Title       string   `json:"title"`
	Link        string   `json:"link"`
	Description string   `json:"description"`
	PubDate     string   `json:"pubDate"`
	Creator     []string `json:"creator"`
	SourceID    string   `json:"source_id"`
	SourceName  string   `json:"source_name"`
	SourceURL   string   `json:"source_url"`
	Language    string   `json:"language"`
}

type response struct {
	Status       string    `json:"status"`
	TotalResults int       `json:"totalResults"`
	Results      []article `json:"results"`
	NextPage     string    `json:"nextPage"`
}

type Source struct {
	client     *http.Client
	endpoint   string
	apiKey     string
	dailyLimit int
	category   string
	quota      quotaStore
	recorder   signalRecorder
	now        func() time.Time

	mu       sync.Mutex
	nextPage string
	cached   []model.Signal
}

func New(client *http.Client, quota quotaStore, recorder signalRecorder, apiKey string, dailyLimit int, category string) *Source {
	return NewWithEndpoint(client, defaultEndpoint, quota, recorder, apiKey, dailyLimit, category)
}

func NewWithEndpoint(client *http.Client, endpoint string, quota quotaStore, recorder signalRecorder, apiKey string, dailyLimit int, category string) *Source {
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	if dailyLimit <= 0 {
		dailyLimit = 200
	}
	return &Source{
		client: client, endpoint: endpoint, apiKey: strings.TrimSpace(apiKey), dailyLimit: dailyLimit,
		category: strings.TrimSpace(category), quota: quota, recorder: recorder,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (s *Source) Name() string { return "NewsData" }

func (s *Source) Discover(ctx context.Context) ([]model.Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.apiKey == "" {
		return cloneSignals(s.cached), nil
	}
	if s.quota == nil {
		return nil, fmt.Errorf("newsdata quota store is required")
	}
	_, allowed, err := s.quota.ReserveDailyQuota(ctx, "newsdata", s.dailyLimit, s.now())
	if err != nil {
		return nil, fmt.Errorf("newsdata quota: %w", err)
	}
	if !allowed {
		return cloneSignals(s.cached), nil
	}

	endpoint, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, err
	}
	params := endpoint.Query()
	params.Set("apikey", s.apiKey)
	params.Set("size", "10")
	params.Set("language", "en")
	params.Set("removeduplicate", "1")
	if s.category != "" {
		params.Set("category", s.category)
	}
	if s.nextPage != "" {
		params.Set("page", s.nextPage)
	}
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
		return nil, fmt.Errorf("newsdata returned HTTP %d", res.StatusCode)
	}

	var payload response
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode newsdata: %w", err)
	}
	if payload.Status != "" && !strings.EqualFold(payload.Status, "success") {
		return nil, fmt.Errorf("newsdata status %q", payload.Status)
	}

	values := make([]model.Signal, 0, len(payload.Results))
	for _, item := range payload.Results {
		link := strings.TrimSpace(item.Link)
		title := strings.TrimSpace(item.Title)
		if link == "" || title == "" {
			continue
		}
		domain := hostname(link)
		name := strings.TrimSpace(item.SourceName)
		if name == "" {
			name = domain
		}
		id := strings.TrimSpace(item.ArticleID)
		if id == "" {
			sum := sha256.Sum256([]byte(link))
			id = hex.EncodeToString(sum[:8])
		}
		author := ""
		if len(item.Creator) > 0 {
			author = strings.TrimSpace(item.Creator[0])
		}
		values = append(values, model.Signal{
			ID:               "newsdata:" + id,
			Source:           model.Source{Name: name, Domain: domain, URL: firstNonEmpty(item.SourceURL, link)},
			DiscoveryChannel: "newsdata",
			Title:            title,
			ClusterText:      title,
			Text:             truncate(strings.TrimSpace(item.Description), 600),
			URL:              link,
			Author:           author,
			PublishedAt:      parsePublished(item.PubDate),
		})
	}
	if len(values) > 0 && s.recorder != nil {
		if err := s.recorder.RecordSignals(ctx, values); err != nil {
			return nil, fmt.Errorf("persist newsdata signals: %w", err)
		}
	}
	s.nextPage = strings.TrimSpace(payload.NextPage)
	s.cached = mergeCache(s.cached, values, 250)
	return cloneSignals(s.cached), nil
}

func (s *Source) QuotaStatus(ctx context.Context) (history.DailyQuotaStatus, error) {
	if s.quota == nil {
		return history.DailyQuotaStatus{}, fmt.Errorf("newsdata quota store is required")
	}
	return s.quota.DailyQuota(ctx, "newsdata", s.dailyLimit, s.now())
}

func mergeCache(existing, incoming []model.Signal, limit int) []model.Signal {
	byID := make(map[string]model.Signal, len(existing)+len(incoming))
	order := make([]string, 0, len(existing)+len(incoming))
	for _, value := range existing {
		if value.ID == "" { continue }
		if _, ok := byID[value.ID]; !ok { order = append(order, value.ID) }
		byID[value.ID] = value
	}
	for _, value := range incoming {
		if value.ID == "" { continue }
		if _, ok := byID[value.ID]; !ok { order = append(order, value.ID) }
		byID[value.ID] = value
	}
	if limit > 0 && len(order) > limit { order = order[len(order)-limit:] }
	out := make([]model.Signal, 0, len(order))
	for _, id := range order { out = append(out, byID[id]) }
	return out
}

func parsePublished(value string) string {
	value = strings.TrimSpace(value)
	if value == "" { return "" }
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil { return parsed.UTC().Format(time.RFC3339) }
	}
	return ""
}

func hostname(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil { return "" }
	return strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" { return strings.TrimSpace(value) }
	}
	return ""
}

func truncate(value string, limit int) string {
	if limit > 0 && len(value) > limit { return strings.TrimSpace(value[:limit]) + "…" }
	return value
}

func cloneSignals(values []model.Signal) []model.Signal {
	out := make([]model.Signal, len(values)); copy(out, values); return out
}
