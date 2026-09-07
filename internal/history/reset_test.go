package history

import (
	"context"
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
