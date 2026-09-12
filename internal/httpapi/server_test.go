package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/buildinfo"
	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/httpapi"
	"github.com/chrisbirster/trendinary/internal/sqlscript"
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
	handler := httpapi.New(store.NewDemoMemory(), http.NotFoundHandler())
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
			TrendKey:   "at-protocol",
			ObservedAt: base.Add(time.Duration(index) * time.Minute),
			Lifecycle:  "RISING",
			Score:      score,
			Raw:        history.RawMetrics{SignalCount: 2 + index, RawAttention: float64(100 + index*50)},
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

func TestHealthReportsRuntimeIdentity(t *testing.T) {
	handler := httpapi.New(
		store.NewMemory(),
		http.NotFoundHandler(),
		httpapi.WithBuildInfo(buildinfo.Info{APIVersion: "v1", Release: "v9.8.7", Commit: "abcdef123456"}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	var payload struct {
		OK         bool   `json:"ok"`
		Version    string `json:"version"`
		APIVersion string `json:"api_version"`
		Release    string `json:"release"`
		Commit     string `json:"commit"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.Version != "v1" || payload.APIVersion != "v1" || payload.Release != "v9.8.7" || payload.Commit != "abcdef123456" {
		t.Fatalf("unexpected health identity: %+v", payload)
	}
}

func TestReadinessHealthyDatabase(t *testing.T) {
	historical := openManagedHistory(t, prepareManagedDatabase(t))
	handler := httpapi.New(
		store.NewMemory(), http.NotFoundHandler(),
		httpapi.WithHistory(historical),
		httpapi.WithBuildInfo(buildinfo.Info{Release: "v9.8.7", Commit: "abc123"}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var payload struct {
		OK       bool   `json:"ok"`
		Database string `json:"database"`
		Release  string `json:"release"`
		Commit   string `json:"commit"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.Database != "ready" || payload.Release != "v9.8.7" || payload.Commit != "abc123" {
		t.Fatalf("unexpected readiness payload: %+v", payload)
	}
}

func TestReadinessRejectsUnavailableDatabaseWithoutLeakingDetails(t *testing.T) {
	historical := openManagedHistory(t, prepareManagedDatabase(t))
	if err := historical.Close(); err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler(), httpapi.WithHistory(historical))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	body := strings.ToLower(res.Body.String())
	for _, forbidden := range []string{"turso_auth_token", "turso_database_url", "password", "libsql://"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("readiness response leaked %q: %s", forbidden, body)
		}
	}
}

func TestReadinessRejectsStaleSchema(t *testing.T) {
	path := prepareManagedDatabase(t)
	admin, err := history.OpenAdminExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.DB().Exec(`DROP TABLE quality_feedback`); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if err := admin.Close(); err != nil {
		t.Fatal(err)
	}
	historical := openManagedHistory(t, path)
	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler(), httpapi.WithHistory(historical))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func prepareManagedDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "managed.db")
	admin, err := history.OpenAdminExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if len(migrations) == 0 {
		_ = admin.Close()
		t.Fatal("no test migrations found")
	}
	for _, migrationPath := range migrations {
		migration, err := os.ReadFile(migrationPath)
		if err != nil {
			_ = admin.Close()
			t.Fatal(err)
		}
		up := strings.SplitN(string(migration), "-- +goose Down", 2)[0]
		if err := sqlscript.Execute(context.Background(), admin.DB(), up); err != nil {
			_ = admin.Close()
			t.Fatalf("apply test migration %s: %v", filepath.Base(migrationPath), err)
		}
	}
	if err := admin.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func openManagedHistory(t *testing.T, path string) *history.Store {
	t.Helper()
	historical, err := history.OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = historical.Close() })
	return historical
}
