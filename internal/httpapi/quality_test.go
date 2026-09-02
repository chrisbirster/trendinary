package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/store"
)

func TestPeepRanksEarlySignalQuality(t *testing.T) {
	memory := store.NewMemory()
	memory.ReplaceTrends([]model.Trend{
		{Slug: "famous", Name: "Famous", Status: "RISING", Score: 95, Quality: model.TrendQuality{PeepScore: 51}},
		{Slug: "slope", Name: "Slope", Status: "EMERGING", Score: 70, Quality: model.TrendQuality{PeepScore: 88}},
		{Slug: "peak", Name: "Peak", Status: "PEAKING", Score: 99, Quality: model.TrendQuality{PeepScore: 99}},
	})
	handler := New(memory, http.NotFoundHandler())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/peep", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct{ Data []model.Trend `json:"data"` }
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 2 || envelope.Data[0].Slug != "slope" || envelope.Data[1].Slug != "famous" {
		t.Fatalf("unexpected PEEP ordering: %+v", envelope.Data)
	}
	if envelope.Data[0].Rank != 1 || envelope.Data[1].Rank != 2 {
		t.Fatalf("PEEP ranks not rewritten: %+v", envelope.Data)
	}
}

func TestFomoUsesPersistedHistory(t *testing.T) {
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()
	now := time.Now().UTC()
	entity, err := historical.ResolveEntity(context.Background(), "Signal Quality", "signal-quality", []string{"signal", "quality"}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := historical.RecordSnapshot(context.Background(), history.Snapshot{
		TrendKey: entity.ID, ObservedAt: now.Add(-30 * time.Minute), Lifecycle: "RISING",
		Score: engine.ScoreBreakdown{Version: engine.ScoreVersion, Score: 82, Velocity: .91, SourceBreadth: .55, Novelty: .8, Confidence: .8},
	}); err != nil {
		t.Fatal(err)
	}
	handler := New(store.NewMemory(), http.NotFoundHandler(), WithHistory(historical))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/fomo?hours=24&limit=7", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct{ Data []history.FomoItem `json:"data"` }
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].Slug != entity.Slug || envelope.Data[0].PeakScore != 82 {
		t.Fatalf("unexpected FOMO data: %+v", envelope.Data)
	}
}
