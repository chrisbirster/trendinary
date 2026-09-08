package history

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestResetDatabaseOnceDropsApplicationStateAndIsIdempotent(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, err := store.DB().ExecContext(ctx, `
CREATE TABLE reset_parent (id INTEGER PRIMARY KEY);
CREATE TABLE reset_child (
  id INTEGER PRIMARY KEY,
  parent_id INTEGER NOT NULL REFERENCES reset_parent(id)
);
INSERT INTO reset_parent(id) VALUES (1);
INSERT INTO reset_child(id, parent_id) VALUES (1, 1);
CREATE VIEW reset_view AS SELECT id FROM reset_parent;
`); err != nil {
		t.Fatal(err)
	}

	applied, err := ResetDatabaseOnce(ctx, store.DB(), "test-clean-slate")
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("first reset was not applied")
	}

	for _, name := range []string{"signals", "trend_snapshots", "reset_parent", "reset_child", "reset_view"} {
		var count int
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("database object %q survived reset", name)
		}
	}

	var markerCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM trendinary_database_resets WHERE reset_id = ?`, "test-clean-slate").Scan(&markerCount); err != nil {
		t.Fatal(err)
	}
	if markerCount != 1 {
		t.Fatalf("reset marker count = %d, want 1", markerCount)
	}

	again, err := ResetDatabaseOnce(ctx, store.DB(), "test-clean-slate")
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("same reset ID was applied twice")
	}

	// Production closes/reopens the Turso Store after an applied reset. Calling
	// migrate directly here verifies that a wiped database can be rebuilt from
	// the current schema without removing the reset ledger.
	if err := store.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"signals", "trend_snapshots", databaseResetTable} {
		var count int
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("rebuilt table %q count = %d, want 1", name, count)
		}
	}
}

func TestResetDatabaseOnceSerializesConcurrentCallers(t *testing.T) {
	store, err := Open(t.TempDir() + "/reset.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.DB().SetMaxOpenConns(4)
	store.DB().SetMaxIdleConns(4)

	ctx := context.Background()
	if _, err := store.DB().ExecContext(ctx, `CREATE TABLE concurrent_reset_data (id INTEGER PRIMARY KEY); INSERT INTO concurrent_reset_data(id) VALUES (1)`); err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		applied bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			applied, err := ResetDatabaseOnce(ctx, store.DB(), "concurrent-clean-slate")
			results <- outcome{applied: applied, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	wins := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.applied {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent reset winners = %d, want exactly 1", wins)
	}

	var markerCount, dataTableCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM trendinary_database_resets WHERE reset_id = ?`, "concurrent-clean-slate").Scan(&markerCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'concurrent_reset_data'`).Scan(&dataTableCount); err != nil {
		t.Fatal(err)
	}
	if markerCount != 1 || dataTableCount != 0 {
		t.Fatalf("marker=%d data_table=%d after concurrent reset", markerCount, dataTableCount)
	}
}

func TestResetDatabaseOnceIgnoresEmptyResetID(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	applied, err := ResetDatabaseOnce(context.Background(), store.DB(), "   ")
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("empty reset ID should be a no-op")
	}

	var signals int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'signals'`).Scan(&signals); err != nil {
		t.Fatal(err)
	}
	if signals != 1 {
		t.Fatalf("signals table count = %d, want 1", signals)
	}
}

func TestResetRetryableConnectionErrorRecognizesClosedLibSQLStreams(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "driver sentinel", err: fmt.Errorf("wrapped: %w", driver.ErrBadConn), want: true},
		{name: "production libsql error", err: errors.New("failed to execute SQL: stream is closed: driver: bad connection"), want: true},
		{name: "stream closed", err: errors.New("stream is closed"), want: true},
		{name: "ordinary SQL error", err: errors.New("no such table: signals"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resetRetryableConnectionError(tt.err); got != tt.want {
				t.Fatalf("resetRetryableConnectionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
