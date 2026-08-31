package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	"github.com/chrisbirster/trendinary/internal/signals"
	"github.com/chrisbirster/trendinary/internal/store"
)

type Server struct {
	store    *store.Memory
	frontend http.Handler
	hn       *hackernews.Client
	bluesky  *bluesky.Client
}

func New(s *store.Memory, frontend http.Handler) http.Handler {
	server := &Server{
		store:    s,
		frontend: frontend,
		hn:       hackernews.NewClient(nil),
		bluesky:  bluesky.NewClient(nil),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/healthz", server.health)
	mux.HandleFunc("GET /api/v1/trends", server.trends)
	mux.HandleFunc("GET /api/v1/trends/{slug}", server.trend)
	mux.HandleFunc("GET /api/v1/signals/hacker-news", server.hackerNewsSignals)
	mux.HandleFunc("GET /api/v1/signals/bluesky", server.blueskySignals)
	mux.HandleFunc("GET /api/v1/sources/{domain}", server.source)
	mux.HandleFunc("GET /api/v1/methodology/bias", server.biasMethodology)
	mux.Handle("/", frontend)
	return withHeaders(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "trendinary",
		"version": "v1",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) trends(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": s.store.Trends()})
}

func (s *Server) trend(w http.ResponseWriter, r *http.Request) {
	trend, ok := s.store.Trend(r.PathValue("slug"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "trend not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": trend})
}

func (s *Server) hackerNewsSignals(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryLimit(w, r, 20, 50)
	if !ok {
		return
	}

	items, err := s.hn.Top(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "hacker news source unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=60")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": signals.HackerNews(items),
		"meta": map[string]any{"source": "Hacker News", "live": true, "normalized": true},
	})
}

func (s *Server) blueskySignals(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "q is required"})
		return
	}
	limit, ok := queryLimit(w, r, 25, 100)
	if !ok {
		return
	}

	result, err := s.bluesky.Search(r.Context(), query, limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "bluesky source unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=15, stale-while-revalidate=30")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": signals.Bluesky(result.Posts),
		"meta": map[string]any{
			"source":     "Bluesky",
			"live":       true,
			"normalized": true,
			"query":      query,
			"hits_total": result.HitsTotal,
		},
	})
}

func (s *Server) source(w http.ResponseWriter, r *http.Request) {
	domain := strings.ToLower(r.PathValue("domain"))
	source, ok := s.store.Source(domain)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "source not rated or not known"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": source})
}

func (s *Server) biasMethodology(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"principle": "Trendinary distinguishes political leaning from factual reliability and does not infer a source-level political label from a single article.",
			"labels": []string{"left", "lean-left", "center", "lean-right", "right", "mixed", "not-rated"},
			"display_rules": []string{
				"Always display the rating provider and confidence when a political leaning label is shown.",
				"Preserve the provider's scope, such as web-only versus TV or opinion coverage.",
				"Use not-rated when no evidence-backed external assessment is available.",
				"Keep source leaning separate from the stance or claims of an individual article.",
			},
			"initial_provider": "AllSides",
		},
	})
}

func queryLimit(w http.ResponseWriter, r *http.Request, defaultValue, maximum int) (int, bool) {
	limit := defaultValue
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maximum {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "limit must be between 1 and " + strconv.Itoa(maximum)})
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
