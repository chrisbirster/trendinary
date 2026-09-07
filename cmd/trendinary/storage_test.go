package main

import (
	"context"
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

func TestResetAndReopenHistoryReopensWinningProcess(t *testing.T) {
	path := t.TempDir() + "/reset.db"
	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	reopenCalls := 0
	fresh, applied, err := resetAndReopenHistory(context.Background(), store, "winner", func() (*history.Store, error) {
		reopenCalls++
		return history.Open(path)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if !applied {
		t.Fatal("first reset should be applied")
	}
	if reopenCalls != 1 {
		t.Fatalf("reopen calls = %d, want 1", reopenCalls)
	}
	assertHistorySchemaPresent(t, fresh)
}

func TestResetAndReopenHistoryReopensProcessThatLostResetRace(t *testing.T) {
	path := t.TempDir() + "/reset-race.db"
	winner, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := history.ResetDatabaseOnce(context.Background(), winner.DB(), "shared-rollout")
	if err != nil {
		winner.Close()
		t.Fatal(err)
	}
	if !applied {
		winner.Close()
		t.Fatal("winner reset should be applied")
	}
	if err := winner.Close(); err != nil {
		t.Fatal(err)
	}

	// This models a second Fly Machine whose Store was opened before it learned
	// that another Machine won the reset. The reset marker is already durable,
	// so ResetDatabaseOnce returns false; the observer must still reopen.
	observer, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	reopenCalls := 0
	fresh, observerApplied, err := resetAndReopenHistory(context.Background(), observer, "shared-rollout", func() (*history.Store, error) {
		reopenCalls++
		return history.Open(path)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if observerApplied {
		t.Fatal("observer should see the reset as already applied")
	}
	if reopenCalls != 1 {
		t.Fatalf("observer reopen calls = %d, want 1", reopenCalls)
	}
	assertHistorySchemaPresent(t, fresh)
}

func assertHistorySchemaPresent(t *testing.T, store *history.Store) {
	t.Helper()
	for _, name := range []string{"signals", "trend_snapshots"} {
		var count int
		if err := store.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %q count = %d, want 1", name, count)
		}
	}
}
