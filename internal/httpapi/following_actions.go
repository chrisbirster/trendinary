package httpapi

import "net/http"

func (s *Server) checkFollowingRadar(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	if _, err := s.following.EvaluateRadar(r.Context(), radarID); err != nil {
		followingError(w, err)
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

func (s *Server) deleteFollowingRadar(w http.ResponseWriter, r *http.Request) {
	if s.following == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "following is not configured"})
		return
	}
	radarID, ok := radarIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.following.Store().DeleteRadar(r.Context(), radarID); err != nil {
		followingError(w, err)
		return
	}
	noStore(w)
	w.WriteHeader(http.StatusNoContent)
}
