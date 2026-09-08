package history

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStoreResetDropsApplicationStateWithoutRecreatingSchema(t *testing.T) {
	store, err := Open(t.TempDir() + "/reset.db")
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

	if err := store.Reset(ctx); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"reset_parent", "reset_child", "reset_view", "trendinary_database_resets",
		"signals", "trend_snapshots",
	} {
		var count int
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("database object %q survived destructive reset", name)
		}
	}
}

func TestStoreResetCanBeRepeated(t *testing.T) {
	store, err := Open(t.TempDir() + "/repeat.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for i := 0; i < 10; i++ {
		if _, err := store.DB().Exec(`CREATE TABLE IF NOT EXISTS disposable (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		if err := store.Reset(context.Background()); err != nil {
			t.Fatalf("reset %d: %v", i+1, err)
		}
	}
}

func TestResetDatabaseSerializesTwentyConcurrentCallers(t *testing.T) {
	store, err := Open(t.TempDir() + "/concurrent.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.DB().SetMaxOpenConns(24)
	store.DB().SetMaxIdleConns(24)

	if _, err := store.DB().Exec(`CREATE TABLE concurrent_reset_data (id INTEGER PRIMARY KEY); INSERT INTO concurrent_reset_data(id) VALUES (1)`); err != nil {
		t.Fatal(err)
	}

	const callers = 20
	start := make(chan struct{})
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			errs <- ResetDatabase(ctx, store.DB())
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	var applicationTables int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&applicationTables); err != nil {
		t.Fatal(err)
	}
	if applicationTables != 0 {
		t.Fatalf("application tables survived concurrent reset: %d", applicationTables)
	}
}

func TestResetRetryableErrorRecognizesBusyAndClosedConnections(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "driver sentinel", err: fmt.Errorf("wrapped: %w", driver.ErrBadConn), want: true},
		{name: "production libsql error", err: errors.New("failed to execute SQL: stream is closed: driver: bad connection"), want: true},
		{name: "sqlite busy", err: errors.New("database is locked (5) (SQLITE_BUSY)"), want: true},
		{name: "table locked", err: errors.New("database table is locked"), want: true},
		{name: "ordinary SQL error", err: errors.New("no such table: signals"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resetRetryableError(tt.err); got != tt.want {
				t.Fatalf("resetRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
