package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/chrisbirster/trendinary/internal/editorial"
)

func (s *adminServer) qualityFeedback(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryLimit(w, r, 100, 1000)
	if !ok {
		return
	}
	values, err := s.service.Store().QualityFeedback(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "quality feedback unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"data": values})
}

func (s *adminServer) putQualityFeedback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TrendKey  string                 `json:"trend_key"`
		TrendName string                 `json:"trend_name"`
		Label     editorial.QualityLabel `json:"label"`
		Note      string                 `json:"note"`
		Score     int                    `json:"score"`
		Lifecycle string                 `json:"lifecycle"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	value, err := s.service.Store().PutQualityFeedback(r.Context(), editorial.QualityFeedback{
		TrendKey: body.TrendKey, TrendSlug: r.PathValue("slug"), TrendName: body.TrendName,
		Label: body.Label, Note: body.Note, Score: body.Score, Lifecycle: body.Lifecycle,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": value})
}

func (s *adminServer) qualityReport(w http.ResponseWriter, r *http.Request) {
	value, err := s.service.Store().QualityReport(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "quality report unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": value,
		"meta": map[string]any{
			"scope": "latest human label per stable trend",
			"note":  "These metrics calibrate detection quality; they do not optimize for clicks or engagement.",
		},
	})
}

func (s *adminServer) qualityReplay(w http.ResponseWriter, r *http.Request) {
	threshold := 0.56
	if raw := strings.TrimSpace(r.URL.Query().Get("threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed <= 0 || parsed > 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "threshold must be > 0 and <= 1"})
			return
		}
		threshold = parsed
	}
	value, err := s.service.Store().QualityReplay(r.Context(), threshold)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": value,
		"meta": map[string]any{"threshold": threshold, "corpus": "persisted signal memberships + latest human labels"},
	})
}
