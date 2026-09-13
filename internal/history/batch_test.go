package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

func TestRecordSignalsBatchUpsertsAndMemberships(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	values := make([]model.Signal, 0, 70)
	for i := 0; i < 70; i++ {
		values = append(values, model.Signal{
			ID:               "signal-" + string(rune('A'+i%26)) + time.Unix(int64(i), 0).UTC().Format("150405"),
			Source:           model.Source{Name: "Example", Domain: "example.com"},
			DiscoveryChannel: "rss",
			Title:            "Example signal",
			Text:             "Example evidence",
			Engagement:       model.Engagement{Score: i},
		})
	}
	if err := store.RecordSignalsBatch(ctx, values); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM signals`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(values) {
		t.Fatalf("signal count = %d, want %d", count, len(values))
	}

	var changesBefore, changesAfter int
	if err := store.db.QueryRowContext(ctx, `SELECT total_changes()`).Scan(&changesBefore); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSignalsBatch(ctx, values); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT total_changes()`).Scan(&changesAfter); err != nil {
		t.Fatal(err)
	}
	if changesAfter != changesBefore {
		t.Fatalf("unchanged signal upsert wrote %d rows, want 0", changesAfter-changesBefore)
	}

	values[0].Engagement.Score = 999
	if err := store.RecordSignalsBatch(ctx, values[:1]); err != nil {
		t.Fatal(err)
	}
	var score int
	if err := store.db.QueryRowContext(ctx, `SELECT score FROM signals WHERE id = ?`, values[0].ID).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 999 {
		t.Fatalf("updated score = %d, want 999", score)
	}

	if err := store.RecordTrendSignalsBatch(ctx, "trend-1", values, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM trend_signal_memberships WHERE trend_key = ?`, "trend-1").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(values) {
		t.Fatalf("membership count = %d, want %d", count, len(values))
	}

	observed := time.Now().UTC()
	if err := store.RecordTrendSignalsBatch(ctx, "trend-2", values[:1], observed); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT total_changes()`).Scan(&changesBefore); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordTrendSignalsBatch(ctx, "trend-2", values[:1], observed.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT total_changes()`).Scan(&changesAfter); err != nil {
		t.Fatal(err)
	}
	if changesAfter != changesBefore {
		t.Fatalf("membership refreshed inside cooldown wrote %d rows, want 0", changesAfter-changesBefore)
	}
}
