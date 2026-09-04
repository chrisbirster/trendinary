package history_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
	_ "modernc.org/sqlite"
)

func TestOpenMigratesLegacySignalsDiscoveryChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE signals (
  id TEXT PRIMARY KEY,
  source_name TEXT NOT NULL,
  source_domain TEXT,
  title TEXT,
  body TEXT,
  url TEXT,
  author TEXT,
  published_at TEXT,
  observed_at TEXT NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  replies INTEGER NOT NULL DEFAULT 0,
  likes INTEGER NOT NULL DEFAULT 0,
  reposts INTEGER NOT NULL DEFAULT 0,
  quotes INTEGER NOT NULL DEFAULT 0
)`)
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	signal := model.Signal{
		ID: "legacy-upgrade-test",
		Source: model.Source{Name: "WIRED", Domain: "wired.com"},
		DiscoveryChannel: "rss",
		Title: "Migration test",
		URL: "https://wired.com/example",
	}
	if err := store.RecordSignals(context.Background(), []model.Signal{signal}); err != nil {
		t.Fatalf("record after migration: %v", err)
	}

	var channel string
	if err := store.DB().QueryRow(`SELECT discovery_channel FROM signals WHERE id = ?`, signal.ID).Scan(&channel); err != nil {
		t.Fatal(err)
	}
	if channel != "rss" {
		t.Fatalf("discovery channel = %q, want rss", channel)
	}
}
