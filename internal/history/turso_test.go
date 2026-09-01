package history

import (
	"strings"
	"testing"
)

func TestOpenTursoRequiresURLAndToken(t *testing.T) {
	if _, err := OpenTurso("", "token"); err == nil || !strings.Contains(err.Error(), "URL is required") {
		t.Fatalf("missing URL error = %v", err)
	}
	if _, err := OpenTurso("libsql://example.turso.io", ""); err == nil || !strings.Contains(err.Error(), "auth token is required") {
		t.Fatalf("missing token error = %v", err)
	}
}

func TestOpenTursoRejectsUnsafeOrUnsupportedSchemesBeforeNetwork(t *testing.T) {
	for _, value := range []string{
		"file:///tmp/trendinary.db",
		"http://example.turso.io",
		"ws://example.turso.io",
		"postgres://example.invalid/db",
	} {
		if _, err := OpenTurso(value, "token"); err == nil || !strings.Contains(err.Error(), "unsupported Turso database URL scheme") {
			t.Fatalf("OpenTurso(%q) error = %v", value, err)
		}
	}
}

func TestBackendDefaultsLocalStoresToSQLite(t *testing.T) {
	store, err := Open(t.TempDir() + "/trendinary.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if got := store.Backend(); got != BackendSQLite {
		t.Fatalf("backend = %q", got)
	}
}
