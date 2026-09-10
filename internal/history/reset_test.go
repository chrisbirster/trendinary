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

func TestStoreResetDropsOnlyTrendinaryOwnedSchemaWithoutRecreatingIt(t *testing.T) {
	store, err := Open(t.TempDir() + "/reset.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, err := store.DB().ExecContext(ctx, `
CREATE TABLE provider_metadata (id INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO provider_metadata(id, value) VALUES (1, 'keep me');
CREATE VIEW provider_metadata_view AS SELECT id, value FROM provider_metadata;
CREATE TABLE IF NOT EXISTS trendinary_database_resets (reset_id TEXT PRIMARY KEY);
CREATE TABLE IF NOT EXISTS goose_db_version (id INTEGER PRIMARY KEY, version_id INTEGER NOT NULL, is_applied INTEGER NOT NULL, tstamp TEXT NOT NULL);
`); err != nil {
		t.Fatal(err)
	}

	if err := store.Reset(ctx); err != nil {
		t.Fatal(err)
	}

	owned := append([]string{}, applicationSchemaTables...)
	owned = append(owned, legacyApplicationSchemaTables...)
	for _, name := range owned {
		var count int
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("Trendinary-owned database object %q survived destructive reset", name)
		}
	}

	var value string
	if err := store.DB().QueryRowContext(ctx, `SELECT value FROM provider_metadata WHERE id = 1`).Scan(&value); err != nil {
		t.Fatalf("unrelated provider table did not survive reset: %v", err)
	}
	if value != "keep me" {
		t.Fatalf("provider value = %q, want keep me", value)
	}
	var viewCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_metadata_view`).Scan(&viewCount); err != nil {
		t.Fatalf("unrelated provider view did not survive reset: %v", err)
	}
	if viewCount != 1 {
		t.Fatalf("provider view count = %d, want 1", viewCount)
	}
}

func TestStoreResetCanBeRepeated(t *testing.T) {
	store, err := Open(t.TempDir() + "/repeat.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for i := 0; i < 10; i++ {
		if _, err := store.DB().Exec(`CREATE TABLE IF NOT EXISTS trendinary_database_resets (reset_id TEXT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		if err := store.Reset(context.Background()); err != nil {
			t.Fatalf("reset %d: %v", i+1, err)
		}
		var count int
		if err := store.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'trendinary_database_resets'`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("legacy application table survived reset %d", i+1)
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

	if _, err := store.DB().Exec(`CREATE TABLE provider_metadata (id INTEGER PRIMARY KEY); INSERT INTO provider_metadata(id) VALUES (1)`); err != nil {
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

	owned := append([]string{}, applicationSchemaTables...)
	owned = append(owned, legacyApplicationSchemaTables...)
	for _, name := range owned {
		var count int
		if err := store.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("Trendinary-owned table %q survived concurrent reset", name)
		}
	}
	var providerRows int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM provider_metadata`).Scan(&providerRows); err != nil {
		t.Fatalf("unrelated provider table did not survive concurrent reset: %v", err)
	}
	if providerRows != 1 {
		t.Fatalf("provider rows = %d, want 1", providerRows)
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
