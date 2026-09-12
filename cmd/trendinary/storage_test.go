package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrisbirster/trendinary/internal/history"
	_ "modernc.org/sqlite"
)

func TestOpenHistoryRequiresTursoWhenConfigured(t *testing.T) {
	t.Setenv("TRENDINARY_REQUIRE_TURSO", "1")
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")

	store, backend, err := openHistory()
	if store != nil {
		store.Close()
		t.Fatal("expected no store")
	}
	if backend != history.BackendTurso {
		t.Fatalf("backend = %q", backend)
	}
	if err == nil || !strings.Contains(err.Error(), "TURSO_DATABASE_URL is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenHistoryRequiresTokenWhenURLConfigured(t *testing.T) {
	t.Setenv("TRENDINARY_REQUIRE_TURSO", "1")
	t.Setenv("TURSO_DATABASE_URL", "libsql://example.turso.io")
	t.Setenv("TURSO_AUTH_TOKEN", "")

	store, backend, err := openHistory()
	if store != nil {
		store.Close()
		t.Fatal("expected no store")
	}
	if backend != history.BackendTurso {
		t.Fatalf("backend = %q", backend)
	}
	if err == nil || !strings.Contains(err.Error(), "TURSO_AUTH_TOKEN is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenHistoryRefusesBlankSQLiteInsteadOfMigratingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blank.db")
	t.Setenv("TRENDINARY_REQUIRE_TURSO", "")
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")
	t.Setenv("TRENDINARY_DB_PATH", path)

	store, backend, err := openHistory()
	if store != nil {
		_ = store.Close()
		t.Fatal("blank database unexpectedly opened")
	}
	if backend != history.BackendSQLite {
		t.Fatalf("backend = %q", backend)
	}
	if err == nil || !strings.Contains(err.Error(), "database schema is not migrated") {
		t.Fatalf("error = %v", err)
	}

	db, openErr := sql.Open("sqlite", path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer db.Close()
	var tables int
	if queryErr := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&tables); queryErr != nil {
		t.Fatal(queryErr)
	}
	if tables != 0 {
		t.Fatalf("normal startup created %d application tables", tables)
	}
}

func TestOpenHistoryUsesGoosePreparedSQLiteForDevelopment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trendinary.db")
	applyTestMigrations(t, path)
	t.Setenv("TRENDINARY_REQUIRE_TURSO", "")
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")
	t.Setenv("TRENDINARY_DB_PATH", path)

	store, backend, err := openHistory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if backend != history.BackendSQLite {
		t.Fatalf("backend = %q", backend)
	}
	if !store.ExternallyManagedSchema() {
		t.Fatal("runtime store was not marked migration-managed")
	}
}

func TestOpenHistoryNeverHonorsLegacyStartupResetEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trendinary.db")
	applyTestMigrations(t, path)
	t.Setenv("TRENDINARY_REQUIRE_TURSO", "")
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")
	t.Setenv("TRENDINARY_DB_PATH", path)

	first, _, err := openHistory()
	if err != nil {
		t.Fatal(err)
	}
	// Use application data rather than schema DDL: runtime schema mutation is
	// intentionally intercepted now that Goose owns schema evolution.
	if _, err := first.DB().Exec(`INSERT INTO signals(id, source_name, discovery_channel, observed_at) VALUES('survivor','test','test','2026-09-08T00:00:00Z')`); err != nil {
		first.Close()
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	// This was the production footgun. Keeping the old variable in an operator's
	// shell or deployment must be harmless forever.
	t.Setenv("TRENDINARY_RESET_DATABASE_ID", "this-must-never-run")
	second, _, err := openHistory()
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	var rows int
	if err := second.DB().QueryRow(`SELECT COUNT(*) FROM signals WHERE id='survivor'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("startup mutated existing data: rows=%d", rows)
	}
}

func applyTestMigrations(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, name := range []string{"00001_baseline.sql", "00002_following_intelligence.sql"} {
		migrationPath := filepath.Join("..", "..", "migrations", name)
		migration, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("apply test migration %s: %v", name, err)
		}
	}
}
