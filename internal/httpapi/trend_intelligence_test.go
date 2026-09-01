package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/httpapi"
	"github.com/chrisbirster/trendinary/internal/recent"
	"github.com/chrisbirster/trendinary/internal/runtimeinfo"
	"github.com/chrisbirster/trendinary/internal/store"
)

func TestStreamHealthReportsRuntimeAndWindow(t *testing.T) {
	status := runtimeinfo.New(true, true)
	status.StreamConnected("https://jetstream.example")
	status.StreamBatch(1234, 17, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC))
	status.ScanSucceeded(time.Date(2026, 9, 1, 10, 1, 0, 0, time.UTC), 120, 14, 8, 1)
	window := recent.New(100, time.Hour)

	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler(), httpapi.WithRuntime(status, window))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/streams", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK { t.Fatalf("status = %d", res.Code) }
	if got := res.Header().Get("Cache-Control"); got != "no-store" { t.Fatalf("cache-control = %q", got) }
	var payload struct {
		Data struct {
			Jetstream runtimeinfo.StreamSnapshot `json:"jetstream"`
			Scanner runtimeinfo.ScannerSnapshot `json:"scanner"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil { t.Fatal(err) }
	if !payload.Data.Jetstream.Connected || payload.Data.Jetstream.LastCursor != 1234 || payload.Data.Scanner.Trends != 8 {
		t.Fatalf("unexpected health payload: %+v", payload.Data)
	}
}

func TestAskTrendIsGroundedAndNoStore(t *testing.T) {
	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trends/at-protocol/ask", bytes.NewBufferString(`{"question":"Why is this trending?"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK { t.Fatalf("status = %d body=%s", res.Code, res.Body.String()) }
	if got := res.Header().Get("Cache-Control"); got != "no-store" { t.Fatalf("cache-control = %q", got) }
	var payload struct {
		Data struct {
			Answer string `json:"answer"`
			Mode string `json:"mode"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil { t.Fatal(err) }
	if payload.Data.Answer == "" || payload.Data.Mode != "grounded-deterministic-v1" {
		t.Fatalf("unexpected ask payload: %+v", payload.Data)
	}
}

func TestStableSlugHistoryResolvesEntityID(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil { t.Fatal(err) }
	defer historical.Close()
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	entity, err := historical.ResolveEntity(ctx, "Stable Topic", "stable-topic-cluster", []string{"stable", "topic"}, now)
	if err != nil { t.Fatal(err) }
	score := engine.Score(engine.ScoreInput{Attention: .5, Velocity: .8, SourceBreadth: .5, CommunityBreadth: .5, Novelty: .7, Confidence: .8})
	if err := historical.RecordSnapshot(ctx, history.Snapshot{TrendKey: entity.ID, ObservedAt: now, Lifecycle: "RISING", Score: score}); err != nil { t.Fatal(err) }

	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler(), httpapi.WithHistory(historical))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/"+entity.Slug+"/history", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK { t.Fatalf("status = %d body=%s", res.Code, res.Body.String()) }
	var payload struct {
		Data []history.Snapshot `json:"data"`
		Meta struct { TrendKey string `json:"trend_key"` } `json:"meta"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil { t.Fatal(err) }
	if len(payload.Data) != 1 || payload.Meta.TrendKey != entity.ID {
		t.Fatalf("stable history lookup failed: %+v", payload)
	}
}
