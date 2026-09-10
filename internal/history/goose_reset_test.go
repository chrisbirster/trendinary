package history

import (
	"context"
	"testing"
)

func TestStoreResetDropsGooseMigrationLedger(t *testing.T) {
	store, err := Open(t.TempDir() + "/goose-reset.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, err := store.DB().ExecContext(ctx, `CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO goose_db_version(version_id, is_applied) VALUES (2, 1)`); err != nil {
		t.Fatal(err)
	}

	if err := store.Reset(ctx); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'goose_db_version'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("goose_db_version survived destructive reset")
	}
}
