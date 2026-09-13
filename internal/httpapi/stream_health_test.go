package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/httpapi"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/runtimeinfo"
	"github.com/chrisbirster/trendinary/internal/store"
)

func TestStreamHealthIncludesSources(t *testing.T) {
	status := runtimeinfo.New(true, true)
	status.SetSources([]runtimeinfo.SourceSnapshot{{
		ID:              "google-trends-us",
		Name:            "Google Trends US",
		Kind:            "rss",
		Enabled:         true,
		Cadence:         15 * time.Minute,
		Successes:       2,
		SignalsProduced: 12,
	}})
	window := recent.New(100, time.Hour)
	handler := httpapi.New(
		store.NewMemory(),
		http.NotFoundHandler(),
		httpapi.WithRuntime(status, window),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/streams", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}

	var payload struct {
		Data struct {
			Sources []runtimeinfo.SourceSnapshot `json:"sources"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(payload.Data.Sources))
	}
	got := payload.Data.Sources[0]
	if got.ID != "google-trends-us" || got.Successes != 2 || got.SignalsProduced != 12 {
		t.Fatalf("unexpected source snapshot: %+v", got)
	}
}
