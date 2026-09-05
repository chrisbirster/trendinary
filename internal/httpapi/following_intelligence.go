package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
)

func (s *Server) followingAlertFeedback(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	var input struct {
		Rating string `json:"rating"`
	}
	if !decodeFollowingJSON(w, r, &input) {
		return
	}
	rating, err := s.following.Store().SetAlertFeedback(r.Context(), radarID, strings.TrimSpace(r.PathValue("id")), input.Rating)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "alert not found"})
		return
	}
	if err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"rating": rating}})
}

func (s *Server) followingAlertFeedbackState(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	feedback, err := s.following.Store().AlertFeedback(r.Context(), radarID)
	if err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{"data": feedback})
}

func (s *Server) followingQuality(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	quality, err := s.following.Store().Quality(r.Context(), radarID)
	if err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{"data": quality})
}

func (s *Server) followingBriefing(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	briefing, err := s.following.Store().Briefing(r.Context(), radarID, time.Now())
	if err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{"data": briefing})
}

func (s *Server) markFollowingBriefingSeen(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.following.Store().MarkBriefingSeen(r.Context(), radarID, time.Now()); err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	w.WriteHeader(http.StatusNoContent)
}
