package httpapi

import (
	"net/http"
	"strings"
	"time"
)

func (s *adminServer) sourceAnalytics(w http.ResponseWriter, r *http.Request) {
	window := 7 * 24 * time.Hour
	if raw := strings.TrimSpace(r.URL.Query().Get("window")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < time.Hour || parsed > 30*24*time.Hour {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "window must be a duration between 1h and 720h"})
			return
		}
		window = parsed
	}
	value, err := s.service.Store().SourceAnalytics(r.Context(), time.Now().UTC().Add(-window))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"data": value,
		"meta": map[string]any{
			"window": window.String(),
			"note": "First-hit and lead-time metrics describe observed discovery timing, not publisher quality or factual reliability.",
		},
	})
}

func (s *adminServer) calibrationV1(w http.ResponseWriter, r *http.Request) {
	value, err := s.service.Store().CalibrationV1(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}
