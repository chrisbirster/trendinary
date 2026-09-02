package editorial

import (
	"context"
	"database/sql"
	"math"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openQualityTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return store
}

func TestQualityReportUsesLatestLabelPerTrend(t *testing.T) {
	store := openQualityTestStore(t)
	ctx := context.Background()

	first, err := store.PutQualityFeedback(ctx, QualityFeedback{
		TrendKey: "trend-1", TrendSlug: "trend-one", TrendName: "Trend One",
		Label: QualityNoise, Score: 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "quality-trend-1-new"
	second.Label = QualityRealTrend
	second.Score = 70
	second.CreatedAt = first.CreatedAt.Add(1)
	if _, err := store.PutQualityFeedback(ctx, second); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutQualityFeedback(ctx, QualityFeedback{
		TrendKey: "trend-2", TrendSlug: "trend-two", TrendName: "Trend Two",
		Label: QualityNoise, Score: 30,
	}); err != nil {
		t.Fatal(err)
	}

	report, err := store.QualityReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Labels != 2 || report.RealTrends != 1 || report.Noise != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if math.Abs(report.PrecisionProxy-0.5) > 0.001 {
		t.Fatalf("precision proxy = %f", report.PrecisionProxy)
	}
	if report.Top10Evaluated != 2 || math.Abs(report.Top10Precision-0.5) > 0.001 {
		t.Fatalf("top-10 precision = %f over %d, want 0.5 over 2", report.Top10Precision, report.Top10Evaluated)
	}
	if math.Abs(report.FalsePositiveRate-0.5) > 0.001 {
		t.Fatalf("false positive rate = %f", report.FalsePositiveRate)
	}
	if report.RecommendedMinScore != 50 {
		t.Fatalf("recommended score = %d, want 50", report.RecommendedMinScore)
	}
}

func TestQualityReportMeasuresSourceBreadthAndLifecycleTiming(t *testing.T) {
	store := openQualityTestStore(t)
	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx, `
CREATE TABLE trend_snapshots (
  trend_key TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  lifecycle TEXT NOT NULL,
  source_breadth REAL NOT NULL,
  source_count INTEGER NOT NULL
)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutQualityFeedback(ctx, QualityFeedback{
		TrendKey: "trend-1", TrendSlug: "trend-one", TrendName: "Trend One",
		Label: QualityRealTrend, Score: 82,
	}); err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	snapshots := []struct {
		at        time.Time
		lifecycle string
		breadth   float64
		sources   int
	}{
		{base, "EMERGING", 0.2, 1},
		{base.Add(30 * time.Minute), "RISING", 0.5, 3},
		{base.Add(90 * time.Minute), "BREAKING", 0.8, 5},
	}
	for _, snapshot := range snapshots {
		if _, err := store.db.ExecContext(ctx, `
INSERT INTO trend_snapshots (trend_key, observed_at, lifecycle, source_breadth, source_count)
VALUES (?, ?, ?, ?, ?)`, "trend-1", snapshot.at.Format(time.RFC3339Nano), snapshot.lifecycle, snapshot.breadth, snapshot.sources); err != nil {
			t.Fatal(err)
		}
	}

	report, err := store.QualityReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(report.AverageSourceBreadth-0.8) > 0.001 || math.Abs(report.AverageSourceCount-5) > 0.001 {
		t.Fatalf("unexpected breadth metrics: %+v", report)
	}
	if math.Abs(report.AverageLeadToBreakingMinutes-90) > 0.001 {
		t.Fatalf("lead to breaking = %f, want 90", report.AverageLeadToBreakingMinutes)
	}
	if math.Abs(report.AverageEmergingToRisingMinutes-30) > 0.001 {
		t.Fatalf("emerging to rising = %f, want 30", report.AverageEmergingToRisingMinutes)
	}
	if math.Abs(report.AverageRisingToBreakingMinutes-60) > 0.001 {
		t.Fatalf("rising to breaking = %f, want 60", report.AverageRisingToBreakingMinutes)
	}
}

func TestQualityFeedbackRejectsUnknownLabel(t *testing.T) {
	store := openQualityTestStore(t)
	_, err := store.PutQualityFeedback(context.Background(), QualityFeedback{
		TrendKey: "trend", TrendSlug: "trend", Label: QualityLabel("maybe"),
	})
	if err == nil {
		t.Fatal("expected unknown quality label to fail")
	}
}
