package httpapi

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/editorial"
)

type publishingNote struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Publisher   string `json:"publisher"`
	ContentType string `json:"contentType"`
	Note        string `json:"note"`
}

// NewIntegration exposes narrowly scoped machine-to-machine endpoints without
// granting callers access to the human /admin credential or the rest of the
// private editorial API.
func NewIntegration(next http.Handler, service *editorial.Service, token string) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/integrations/notes" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if service == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "editorial integration is not configured"})
			return
		}
		if strings.TrimSpace(token) == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "TRENDINARY_INTEGRATION_TOKEN is required to enable integrations"})
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "integration authentication required"})
			return
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("worth_sharing")); raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil || !value {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "integration notes only supports worth_sharing=true"})
				return
			}
		}
		approved := true
		items, err := listEditorialNotes(r.Context(), service, &approved)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "integration notes unavailable"})
			return
		}
		out := make([]publishingNote, 0, len(items))
		for _, item := range items {
			url := item.CanonicalURL
			if url == "" {
				url = item.OriginalURL
			}
			out = append(out, publishingNote{
				ID:          item.ID,
				Title:       item.Title,
				URL:         url,
				Publisher:   item.Publisher,
				ContentType: string(item.ContentType),
				Note:        item.Note.Text,
			})
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, out)
	})
}

func listEditorialNotes(ctx context.Context, service *editorial.Service, worthSharing *bool) ([]editorial.ContentItem, error) {
	if service == nil {
		return nil, fmt.Errorf("editorial service is required")
	}
	values, err := service.Store().ListContent(ctx, "", "", false, "newest", time.Time{}, 500)
	if err != nil {
		return nil, err
	}
	out := make([]editorial.ContentItem, 0)
	for _, item := range values {
		if item.Note == nil || (item.State != editorial.StateConsumed && item.State != editorial.StateSaved) {
			continue
		}
		if worthSharing != nil {
			if item.Note.WorthSharing == nil || *item.Note.WorthSharing != *worthSharing {
				continue
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func optionalBoolQuery(r *http.Request, name string) (*bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be true or false", name)
	}
	return &value, nil
}
