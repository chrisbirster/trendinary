package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/following"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/ingest/bluesky"
	"github.com/chrisbirster/trendinary/internal/ingest/hackernews"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/runtimeinfo"
	"github.com/chrisbirster/trendinary/internal/signals"
	"github.com/chrisbirster/trendinary/internal/store"
)

type Server struct {
	store         *store.Memory
	frontend      http.Handler
	hn            *hackernews.Client
	bluesky       *bluesky.Client
	history       *history.Store
	runtime       *runtimeinfo.Status
	recent        *recent.Store
	following     *following.Service
	followingPush *following.PushSender
}

type Option func(*Server)

func WithHistory(historical *history.Store) Option {
	return func(server *Server) {
		server.history = historical
	}
}

func WithRuntime(status *runtimeinfo.Status, recentSignals *recent.Store) Option {
	return func(server *Server) {
		server.runtime = status
		server.recent = recentSignals
	}
}

func WithFollowing(service *following.Service, push *following.PushSender) Option {
	return func(server *Server) {
		server.following = service
		server.followingPush = push
	}
}

func New(s *store.Memory, frontend http.Handler, options ...Option) http.Handler {
	server := &Server{
		store:    s,
		frontend: frontend,
		hn:       hackernews.NewClient(nil),
		bluesky:  bluesky.NewClient(nil),
	}
	for _, option := range options {
		if option != nil {
			option(server)
		}
	}
	// Production main supplies both durable history and runtime status. That is
	// the boundary where v0.5's anonymous server-backed radar becomes active.
	// Most focused handler tests omit runtime status, so they do not leak a
	// minute ticker merely by constructing a server with a temporary history DB.
	if server.following == nil && server.history != nil && server.runtime != nil {
		if radarStore, err := following.NewStore(server.history.DB()); err != nil {
			slog.Error("configure following store", "error", err)
		} else if push, err := following.NewPushSender(context.Background(), radarStore, nil); err != nil {
			slog.Error("configure following Web Push", "error", err)
		} else {
			server.followingPush = push
			server.following = following.NewService(radarStore, s, push)
			go runFollowingWorker(server.following, server.runtime)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/healthz", server.health)
	mux.HandleFunc("GET /api/v1/health/streams", server.streamHealth)
	mux.HandleFunc("GET /api/v1/trends", server.trends)
	mux.HandleFunc("GET /api/v1/peep", server.peep)
	mux.HandleFunc("GET /api/v1/fomo", server.fomo)
	mux.HandleFunc("POST /api/v1/following/radar", server.createFollowingRadar)
	mux.HandleFunc("DELETE /api/v1/following/radar", server.deleteFollowingRadar)
	mux.HandleFunc("GET /api/v1/following/state", server.followingState)
	mux.HandleFunc("POST /api/v1/following/check", server.checkFollowingRadar)
	mux.HandleFunc("POST /api/v1/following/follows", server.addFollowingFollow)
	mux.HandleFunc("DELETE /api/v1/following/follows/{id}", server.removeFollowingFollow)
	mux.HandleFunc("PATCH /api/v1/following/preferences", server.updateFollowingPreferences)
	mux.HandleFunc("POST /api/v1/following/alerts/read", server.markFollowingRead)
	mux.HandleFunc("DELETE /api/v1/following/alerts", server.clearFollowingAlerts)
	mux.HandleFunc("GET /api/v1/following/push/public-key", server.followingPushPublicKey)
	mux.HandleFunc("PUT /api/v1/following/push/subscription", server.putFollowingPushSubscription)
	mux.HandleFunc("DELETE /api/v1/following/push/subscription", server.deleteFollowingPushSubscription)
	mux.HandleFunc("GET /api/v1/trends/{slug}/history", server.trendHistory)
	mux.HandleFunc("GET /api/v1/trends/{slug}/propagation", server.trendPropagation)
	mux.HandleFunc("GET /api/v1/trends/{slug}/explanation", server.trendExplanation)
	mux.HandleFunc("POST /api/v1/trends/{slug}/ask", server.askTrend)
	mux.HandleFunc("GET /api/v1/trends/{slug}", server.trend)
	mux.HandleFunc("GET /api/v1/signals/hacker-news", server.hackerNewsSignals)
	mux.HandleFunc("GET /api/v1/signals/bluesky", server.blueskySignals)
	mux.HandleFunc("GET /api/v1/sources/{domain}", server.source)
	mux.HandleFunc("GET /api/v1/methodology/bias", server.biasMethodology)
	mux.HandleFunc("GET /api/v1/methodology/score", server.scoreMethodology)
	mux.Handle("/", frontend)
	return withHeaders(mux)
}

func runFollowingWorker(service *following.Service, status *runtimeinfo.Status) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		// Memory starts with demo/sample trends so the static app and focused
		// handler tests remain useful before ingestion. Never let those values
		// become durable radar baselines after a production restart: Following
		// stays dormant until the scanner has successfully published real data.
		if status == nil || status.Snapshot().Scanner.LastSuccessAt.IsZero() {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := service.Evaluate(ctx)
		cancel()
		if err != nil {
			slog.Warn("following evaluation failed", "error", err)
			continue
		}
		if result.Alerts > 0 {
			slog.Info("following alerts generated", "radars", result.Radars, "follows", result.Follows, "matches", result.Matches, "alerts", result.Alerts, "push_attempts", result.PushAttempts)
		}
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"service":   "trendinary",
		"version":   "v1",
		"time":      time.Now().UTC().Format(time.RFC3339),
		"history":   s.history != nil,
		"following": s.following != nil,
		"web_push":  s.followingPush != nil && s.followingPush.PublicKey() != "",
	})
}

func (s *Server) streamHealth(w http.ResponseWriter, r *http.Request) {
	var snapshot runtimeinfo.Snapshot
	if s.runtime != nil {
		snapshot = s.runtime.Snapshot()
	}
	windowSize := 0
	if s.recent != nil {
		windowSize = s.recent.Len()
	}
	if s.history != nil && snapshot.Stream.LastCursor == 0 {
		if cursor, ok, err := s.history.Cursor(r.Context(), "atproto-jetstream-v2-posts"); err == nil && ok {
			snapshot.Stream.LastCursor = cursor
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"jetstream": snapshot.Stream,
			"scanner":   snapshot.Scanner,
			"window": map[string]any{
				"signals": windowSize,
			},
		},
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

func (s *Server) trendHistory(w http.ResponseWriter, r *http.Request) {
	if s.history == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "trend history is not configured"})
		return
	}
	limit, ok := queryLimit(w, r, 48, 500)
	if !ok {
		return
	}
	trendKey, err := s.history.ResolveTrendKey(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "trend identity unavailable"})
		return
	}
	snapshots, err := s.history.RecentSnapshots(r.Context(), trendKey, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "trend history unavailable"})
		return
	}
	for left, right := 0, len(snapshots)-1; left < right; left, right = left+1, right-1 {
		snapshots[left], snapshots[right] = snapshots[right], snapshots[left]
	}
	w.Header().Set("Cache-Control", "public, max-age=15, stale-while-revalidate=30")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": snapshots,
		"meta": map[string]any{"trend_key": trendKey, "slug": r.PathValue("slug"), "score_version": engine.ScoreVersion},
	})
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

func (s *Server) scoreMethodology(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"version": engine.ScoreVersion,
			"range":   "0-100",
			"principle": "Score unexpected attention, not fame. V3 shifts weight from absolute attention toward velocity and independent-source breadth, then measures quality against durable human labels.",
			"weights": map[string]float64{
				"attention":         0.14,
				"velocity":          0.34,
				"source_breadth":    0.19,
				"community_breadth": 0.13,
				"novelty":           0.12,
				"confidence":        0.08,
			},
			"peep": "PEEP is a separate confidence-gated early-signal score emphasizing velocity, source breadth, community breadth, and novelty.",
			"calibration": "Private human labels feed precision, timeliness, cluster-health, naming-health, threshold recommendations, and deterministic replay evaluation.",
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