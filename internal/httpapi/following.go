package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/chrisbirster/trendinary/internal/following"
)

const followingBodyLimit = 32 << 10

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func (s *Server) createFollowingRadar(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	token, state, err := s.following.Store().CreateRadar(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "unable to create radar"})
		return
	}
	noStore(w)
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"sync_key": token, "state": state}})
}

func radarIDFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authorization, "Bearer ") {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Trendinary Radar"`)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "radar key is required"})
		return "", false
	}
	id, err := following.RadarID(strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid radar key"})
		return "", false
	}
	return id, true
}

func followingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, following.ErrRadarNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "radar not found"})
	case errors.Is(err, following.ErrInvalidRadarKey):
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid radar key"})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
}

func decodeFollowingJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	reader := io.LimitReader(r.Body, followingBodyLimit)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return false
	}
	return true
}

func (s *Server) followingState(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	state, err := s.following.Store().State(r.Context(), radarID)
	if err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{"data": state})
}

func (s *Server) addFollowingFollow(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	var input struct {
		Kind        string `json:"kind"`
		Value       string `json:"value"`
		DisplayName string `json:"display_name"`
	}
	if !decodeFollowingJSON(w, r, &input) {
		return
	}
	follow, err := s.following.Store().AddFollow(r.Context(), radarID, input.Kind, input.Value, input.DisplayName)
	if err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusCreated, map[string]any{"data": follow})
}

func (s *Server) removeFollowingFollow(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.following.Store().RemoveFollow(r.Context(), radarID, strings.TrimSpace(r.PathValue("id"))); err != nil {
		if errors.Is(err, following.ErrRadarNotFound) {
			followingError(w, err)
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "follow not found"})
		return
	}
	noStore(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateFollowingPreferences(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	var input following.Preferences
	if !decodeFollowingJSON(w, r, &input) {
		return
	}
	value, err := s.following.Store().UpdatePreferences(r.Context(), radarID, input)
	if err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (s *Server) markFollowingRead(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.following.Store().MarkAlertsRead(r.Context(), radarID); err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) clearFollowingAlerts(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.following.Store().ClearAlerts(r.Context(), radarID); err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) followingPushPublicKey(w http.ResponseWriter, _ *http.Request) {
	if s.followingPush == nil || s.followingPush.PublicKey() == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Web Push is not configured"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"public_key": s.followingPush.PublicKey()}})
}

func (s *Server) putFollowingPushSubscription(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	var input following.PushSubscription
	if !decodeFollowingJSON(w, r, &input) {
		return
	}
	value, err := s.following.Store().RebindPushSubscription(r.Context(), radarID, input)
	if err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"id": value.ID}})
}

func (s *Server) deleteFollowingPushSubscription(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	var input struct {
		Endpoint string `json:"endpoint"`
	}
	if !decodeFollowingJSON(w, r, &input) {
		return
	}
	if err := s.following.Store().DeletePushSubscription(r.Context(), radarID, input.Endpoint); err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	w.WriteHeader(http.StatusNoContent)
}
