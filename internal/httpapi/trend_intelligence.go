package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/explain"
)

func (s *Server) trendPropagation(w http.ResponseWriter, r *http.Request) {
	trend, ok := s.store.Trend(r.PathValue("slug"))
	if ok && len(trend.Propagation) > 0 {
		writeJSON(w, http.StatusOK, map[string]any{"data": trend.Propagation})
		return
	}
	if s.history == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "propagation history is not configured"})
		return
	}
	trendKey, err := s.history.ResolveTrendKey(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "trend identity unavailable"})
		return
	}
	path, err := s.history.Propagation(r.Context(), trendKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "propagation unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": path})
}

func (s *Server) trendExplanation(w http.ResponseWriter, r *http.Request) {
	trend, ok := s.store.Trend(r.PathValue("slug"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "trend not found"})
		return
	}
	if trend.Explanation == nil {
		fallback := explain.Build(trend, nil, time.Time{})
		trend.Explanation = &fallback
	}
	w.Header().Set("Cache-Control", "public, max-age=15, stale-while-revalidate=30")
	writeJSON(w, http.StatusOK, map[string]any{"data": trend.Explanation})
}

func (s *Server) askTrend(w http.ResponseWriter, r *http.Request) {
	trend, ok := s.store.Trend(r.PathValue("slug"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "trend not found"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	defer r.Body.Close()
	var request struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "valid JSON body is required"})
		return
	}
	request.Question = strings.TrimSpace(request.Question)
	if request.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "question is required"})
		return
	}
	if len(request.Question) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "question must be 500 characters or fewer"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"data": explain.AnswerQuestion(request.Question, trend)})
}
