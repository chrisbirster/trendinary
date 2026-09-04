package history_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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

func TestRecordTrendSignalsMigratesLegacyMembershipFirstObservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-membership.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
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
);
CREATE TABLE trend_signal_memberships (
  trend_key TEXT NOT NULL,
  signal_id TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY (trend_key, signal_id)
);
INSERT INTO signals (id, source_name, source_domain, title, body, url, author, published_at, observed_at)
VALUES ('wired:legacy', 'WIRED', 'wired.com', 'Audacity 4.0', '', 'https://wired.com/audacity', '', '', '` + first.Format(time.RFC3339Nano) + `');
INSERT INTO trend_signal_memberships (trend_key, signal_id, observed_at)
VALUES ('trend-audacity', 'wired:legacy', '` + first.Format(time.RFC3339Nano) + `');`)
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

	signal := model.Signal{ID: "wired:legacy", Source: model.Source{Name: "WIRED", Domain: "wired.com"}, DiscoveryChannel: "rss", Title: "Audacity 4.0"}
	last := first.Add(2 * time.Hour)
	if err := store.RecordSignals(context.Background(), []model.Signal{signal}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordTrendSignals(context.Background(), "trend-audacity", []model.Signal{signal}, last); err != nil {
		t.Fatal(err)
	}

	var firstRaw, lastRaw string
	if err := store.DB().QueryRow(`SELECT first_observed_at, observed_at FROM trend_signal_memberships WHERE trend_key = ? AND signal_id = ?`, "trend-audacity", signal.ID).Scan(&firstRaw, &lastRaw); err != nil {
		t.Fatal(err)
	}
	firstGot, err := time.Parse(time.RFC3339Nano, firstRaw)
	if err != nil {
		t.Fatal(err)
	}
	lastGot, err := time.Parse(time.RFC3339Nano, lastRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !firstGot.Equal(first) || !lastGot.Equal(last) {
		t.Fatalf("migrated membership first=%s last=%s, want %s / %s", firstGot, lastGot, first, last)
	}
}
