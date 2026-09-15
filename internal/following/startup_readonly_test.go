package following

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/chrisbirster/trendinary/internal/history"
	_ "modernc.org/sqlite"
)

func TestNewPushSenderDoesNotProvisionVAPIDKeyOnManagedRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "managed.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE following_kv (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
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
	_, err = NewPushSender(context.Background(), store, nil)
	if !errors.Is(err, errVAPIDKeyNotInitialized) {
		t.Fatalf("NewPushSender() error = %v, want %v", err, errVAPIDKeyNotInitialized)
	}

	var count int
	if err := historical.DB().QueryRow(`SELECT COUNT(*) FROM following_kv`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("runtime startup inserted %d VAPID row(s), want 0", count)
	}
}
