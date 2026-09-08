package main

import (
	"strings"
	"testing"

	"github.com/chrisbirster/trendinary/internal/history"
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

func TestOpenHistoryFallsBackToLocalSQLiteForDevelopment(t *testing.T) {
	t.Setenv("TRENDINARY_REQUIRE_TURSO", "")
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")
	t.Setenv("TRENDINARY_DB_PATH", t.TempDir()+"/trendinary.db")

	store, backend, err := openHistory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if backend != history.BackendSQLite {
		t.Fatalf("backend = %q", backend)
	}
}

func TestOpenHistoryNeverHonorsLegacyStartupResetEnvironment(t *testing.T) {
	path := t.TempDir() + "/trendinary.db"
	t.Setenv("TRENDINARY_REQUIRE_TURSO", "")
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")
	t.Setenv("TRENDINARY_DB_PATH", path)

	first, _, err := openHistory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.DB().Exec(`CREATE TABLE must_survive_startup (id INTEGER PRIMARY KEY); INSERT INTO must_survive_startup(id) VALUES (1)`); err != nil {
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
	if err := second.DB().QueryRow(`SELECT COUNT(*) FROM must_survive_startup`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("startup mutated existing data: rows=%d", rows)
	}
}
