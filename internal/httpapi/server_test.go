package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
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

func TestTrendHistoryIsChronological(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()

	ctx := context.Background()
	base := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	for index, scoreValue := range []int{40, 70} {
		score := engine.Score(engine.ScoreInput{Attention: float64(scoreValue) / 100, Velocity: 0.7, SourceBreadth: 0.5, CommunityBreadth: 0.5, Novelty: 0.8, Confidence: 0.8})
		if err := historical.RecordSnapshot(ctx, history.Snapshot{
			TrendKey: "at-protocol",
			ObservedAt: base.Add(time.Duration(index) * time.Minute),
			Lifecycle: "RISING",
			Score: score,
			Raw: history.RawMetrics{SignalCount: 2 + index, RawAttention: float64(100 + index*50)},
		}); err != nil {
			t.Fatal(err)
		}
	}

	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler(), httpapi.WithHistory(historical))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/at-protocol/history?limit=10", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var payload struct {
		Data []history.Snapshot `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("history length = %d, want 2", len(payload.Data))
	}
	if !payload.Data[0].ObservedAt.Before(payload.Data[1].ObservedAt) {
		t.Fatalf("history should be chronological: %+v", payload.Data)
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
