package httpapi

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

func (s *Server) peep(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryLimit(w, r, 12, 50)
	if !ok {
		return
	}
	values := s.store.Trends()
	out := make([]model.Trend, 0, len(values))
	for _, trend := range values {
		status := strings.ToUpper(trend.Status)
		if status != "EMERGING" && status != "RISING" {
			continue
		}
		out = append(out, trend)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Quality.PeepScore == out[j].Quality.PeepScore {
			if out[i].Score == out[j].Score {
				return out[i].Slug < out[j].Slug
			}
			return out[i].Score > out[j].Score
		}
		return out[i].Quality.PeepScore > out[j].Quality.PeepScore
	})
	if len(out) > limit {
		out = out[:limit]
	}
	for i := range out {
		out[i].Rank = i + 1
	}
	w.Header().Set("Cache-Control", "public, max-age=10, stale-while-revalidate=20")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": out,
		"meta": map[string]any{
			"principle": "PEEP ranks abnormal slope and spread before raw popularity.",
			"score":     "velocity + source breadth + community breadth + novelty, confidence-gated",
		},
	})
}

func (s *Server) fomo(w http.ResponseWriter, r *http.Request) {
	if s.history == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "trend history is not configured"})
		return
	}
	limit, ok := queryLimit(w, r, 7, 25)
	if !ok {
		return
	}
	hours := 24
	if raw := strings.TrimSpace(r.URL.Query().Get("hours")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 168 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "hours must be between 1 and 168"})
			return
		}
		hours = parsed
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	items, err := s.history.FomoBriefing(r.Context(), since, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "FOMO briefing unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=60")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]any{
			"hours": hours,
			"limit": limit,
			"rule":  "finite history-backed catch-up; no infinite feed",
		},
	})
}
