package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/httpapi"
	"github.com/chrisbirster/trendinary/internal/store"
)

func TestHealth(t *testing.T) {
	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestTrend(t *testing.T) {
	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/at-protocol", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var payload struct {
		Data struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Slug != "at-protocol" {
		t.Fatalf("slug = %q", payload.Data.Slug)
	}
}

func TestBiasMetadataPreservesProvider(t *testing.T) {
	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/foxnews.com", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var payload struct {
		Data struct {
			Bias struct {
				Label    string `json:"label"`
				Provider string `json:"provider"`
			} `json:"bias"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Bias.Label != "right" || payload.Data.Bias.Provider != "AllSides" {
		t.Fatalf("unexpected bias metadata: %+v", payload.Data.Bias)
	}
}

func TestScoreMethodologyIsVersionedAndTransparent(t *testing.T) {
	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/methodology/score", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var payload struct {
		Data struct {
			Version string             `json:"version"`
			Weights map[string]float64 `json:"weights"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Version != engine.ScoreVersion {
		t.Fatalf("version = %q, want %q", payload.Data.Version, engine.ScoreVersion)
	}
	if payload.Data.Weights["velocity"] <= payload.Data.Weights["attention"] {
		t.Fatalf("velocity should outweigh raw attention: %+v", payload.Data.Weights)
	}
}
