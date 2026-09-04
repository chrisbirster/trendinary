package editorial

import (
	"context"
	"fmt"
	"testing"
)

func prepareCalibrationSignalsTable(t *testing.T, store *Store) {
	t.Helper()
	if _, err := store.db.Exec(`
CREATE TABLE signals (
 id TEXT PRIMARY KEY,
 source_name TEXT NOT NULL,
 source_domain TEXT,
 title TEXT,
 body TEXT,
 url TEXT,
 author TEXT,
 published_at TEXT,
 score INTEGER NOT NULL DEFAULT 0,
 replies INTEGER NOT NULL DEFAULT 0,
 likes INTEGER NOT NULL DEFAULT 0,
 reposts INTEGER NOT NULL DEFAULT 0,
 quotes INTEGER NOT NULL DEFAULT 0
);`); err != nil {
		t.Fatal(err)
	}
}

func TestCalibrationV1RefusesToTuneSmallHumanSample(t *testing.T) {
	store := openQualityTestStore(t)
	prepareCalibrationSignalsTable(t, store)
	ctx := context.Background()
	for index := 0; index < 5; index++ {
		if _, err := store.PutQualityFeedback(ctx, QualityFeedback{
			TrendKey: fmt.Sprintf("trend-%d", index), TrendSlug: fmt.Sprintf("trend-%d", index),
			TrendName: "Trend", Label: QualityRealTrend, Score: 60,
		}); err != nil {
			t.Fatal(err)
		}
	}
	value, err := store.CalibrationV1(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if value.Ready {
		t.Fatal("calibration unexpectedly ready with five labels")
	}
	if value.Labels != 5 || value.RequiredLabels != 100 {
		t.Fatalf("calibration progress = %+v", value)
	}
	if value.RecommendedClusterThreshold != nil {
		t.Fatalf("small sample produced threshold recommendation: %+v", value)
	}
}
