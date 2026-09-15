package editorial

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/chrisbirster/trendinary/internal/history"
	_ "modernc.org/sqlite"
)

func TestNewServiceDoesNotWriteExternallyManagedSourceMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "managed.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE editorial_sources (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		url TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		last_attempted_at TEXT,
		last_successful_at TEXT,
		latest_item_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	historical, err := history.OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	defer historical.Close()
	store, err := NewStore(historical.DB())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(store, nil); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := historical.DB().QueryRow(`SELECT COUNT(*) FROM editorial_sources`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("runtime startup inserted %d editorial source row(s), want 0", count)
	}
}
