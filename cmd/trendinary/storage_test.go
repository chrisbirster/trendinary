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
