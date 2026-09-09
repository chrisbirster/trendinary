package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFollowingIntelligenceRoutesArePrivateAndNoStore(t *testing.T) {
	handler := followingHandler(t)
	unauthenticated := httptest.NewRequest(http.MethodGet, "/api/v1/following/briefing", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedResponse, unauthenticated)
	if unauthenticatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("briefing without key status=%d", unauthenticatedResponse.Code)
	}

	key := createRadarKey(t, handler)
	for _, path := range []string{
		"/api/v1/following/alerts/feedback",
		"/api/v1/following/quality",
		"/api/v1/following/briefing",
	} {
		request := authenticatedRequest(http.MethodGet, path, key, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s cache-control=%q", path, response.Header().Get("Cache-Control"))
		}
	}

	briefing := authenticatedRequest(http.MethodGet, "/api/v1/following/briefing", key, nil)
	briefingResponse := httptest.NewRecorder()
	handler.ServeHTTP(briefingResponse, briefing)
	var payload struct {
		Data struct {
			Quiet bool `json:"quiet"`
		} `json:"data"`
	}
	if err := json.Unmarshal(briefingResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Data.Quiet {
		t.Fatal("new radar briefing should be quiet")
	}

	seen := authenticatedRequest(http.MethodPost, "/api/v1/following/briefing/seen", key, nil)
	seenResponse := httptest.NewRecorder()
	handler.ServeHTTP(seenResponse, seen)
	if seenResponse.Code != http.StatusNoContent {
		t.Fatalf("mark briefing seen status=%d body=%s", seenResponse.Code, seenResponse.Body.String())
	}
}

func TestFollowingFeedbackRejectsUnknownAlert(t *testing.T) {
	handler := followingHandler(t)
	key := createRadarKey(t, handler)
	request := authenticatedRequest(http.MethodPost, "/api/v1/following/alerts/missing/feedback", key, []byte(`{"rating":"useful"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("feedback unknown alert status=%d body=%s", response.Code, response.Body.String())
	}
}
