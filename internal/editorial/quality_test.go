package editorial

import (
	"context"
	"database/sql"
	"math"
	"testing"

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
	if report.RecommendedMinScore != 50 {
		t.Fatalf("recommended score = %d, want 50", report.RecommendedMinScore)
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
