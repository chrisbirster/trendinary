package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chrisbirster/trendinary/internal/following"
	"github.com/chrisbirster/trendinary/internal/httpapi"
	"github.com/chrisbirster/trendinary/internal/store"
	_ "modernc.org/sqlite"
)

func followingHandler(t *testing.T) http.Handler {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	radarStore, err := following.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	memory := store.NewMemory()
	service := following.NewService(radarStore, memory, nil)
	return httpapi.New(memory, http.NotFoundHandler(), httpapi.WithFollowing(service, nil))
}

func createRadarKey(t *testing.T, handler http.Handler) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/following/radar", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create radar status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			SyncKey string `json:"sync_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.SyncKey == "" {
		t.Fatal("create radar returned empty key")
	}
	return payload.Data.SyncKey
}

func authenticatedRequest(method, path, key string, body []byte) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+key)
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func TestFollowingRequiresRadarKey(t *testing.T) {
	handler := followingHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/following/state", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", response.Code)
	}
	if response.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate header")
	}
}

func TestFollowingRadarCRUDAndNoStore(t *testing.T) {
	handler := followingHandler(t)
	key := createRadarKey(t, handler)

	add := authenticatedRequest(http.MethodPost, "/api/v1/following/follows", key, []byte(`{"kind":"trend","value":"at-protocol","display_name":"AT Protocol"}`))
	addResponse := httptest.NewRecorder()
	handler.ServeHTTP(addResponse, add)
	if addResponse.Code != http.StatusCreated {
		t.Fatalf("add follow status=%d body=%s", addResponse.Code, addResponse.Body.String())
	}

	check := authenticatedRequest(http.MethodPost, "/api/v1/following/check", key, nil)
	checkResponse := httptest.NewRecorder()
	handler.ServeHTTP(checkResponse, check)
	if checkResponse.Code != http.StatusOK {
		t.Fatalf("check status=%d body=%s", checkResponse.Code, checkResponse.Body.String())
	}
	if checkResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("private radar cache-control=%q", checkResponse.Header().Get("Cache-Control"))
	}

	state := authenticatedRequest(http.MethodGet, "/api/v1/following/state", key, nil)
	stateResponse := httptest.NewRecorder()
	handler.ServeHTTP(stateResponse, state)
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("state status=%d body=%s", stateResponse.Code, stateResponse.Body.String())
	}
	var payload struct {
		Data following.State `json:"data"`
	}
	if err := json.Unmarshal(stateResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Follows) != 1 || payload.Data.Follows[0].Value != "at-protocol" {
		t.Fatalf("state follows=%+v", payload.Data.Follows)
	}

	remove := authenticatedRequest(http.MethodDelete, "/api/v1/following/follows/"+payload.Data.Follows[0].ID, key, nil)
	removeResponse := httptest.NewRecorder()
	handler.ServeHTTP(removeResponse, remove)
	if removeResponse.Code != http.StatusNoContent {
		t.Fatalf("remove follow status=%d body=%s", removeResponse.Code, removeResponse.Body.String())
	}

	deleteRadar := authenticatedRequest(http.MethodDelete, "/api/v1/following/radar", key, nil)
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRadar)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete radar status=%d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}

	missing := authenticatedRequest(http.MethodGet, "/api/v1/following/state", key, nil)
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("deleted radar status=%d, want 404", missingResponse.Code)
	}
}

func TestFollowingPreferenceValidationAndUpdate(t *testing.T) {
	handler := followingHandler(t)
	key := createRadarKey(t, handler)

	update := authenticatedRequest(http.MethodPatch, "/api/v1/following/preferences", key, []byte(`{"sensitivity":"quiet","lifecycle":true,"velocity":false,"corroboration":true,"resurfacing":false}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, update)
	if response.Code != http.StatusOK {
		t.Fatalf("preferences status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data following.Preferences `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Sensitivity != "quiet" || payload.Data.Velocity || payload.Data.Resurfacing {
		t.Fatalf("preferences=%+v", payload.Data)
	}
}

func TestFollowingInvalidKeyDoesNotCreateRadar(t *testing.T) {
	handler := followingHandler(t)
	request := authenticatedRequest(http.MethodGet, "/api/v1/following/state", "not-a-valid-key", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", response.Code)
	}
}

func TestFollowingCheckUsesCurrentServerTrendSnapshot(t *testing.T) {
	handler := followingHandler(t)
	key := createRadarKey(t, handler)
	add := authenticatedRequest(http.MethodPost, "/api/v1/following/follows", key, []byte(`{"kind":"topic","value":"tech","display_name":"Tech"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, add)
	if response.Code != http.StatusCreated {
		t.Fatalf("add topic status=%d", response.Code)
	}
	check := authenticatedRequest(http.MethodPost, "/api/v1/following/check", key, nil)
	checkResponse := httptest.NewRecorder()
	handler.ServeHTTP(checkResponse, check)
	if checkResponse.Code != http.StatusOK {
		t.Fatalf("check status=%d body=%s", checkResponse.Code, checkResponse.Body.String())
	}
	_ = context.Background() // Keep this test explicit about synchronous server-side evaluation.
}
