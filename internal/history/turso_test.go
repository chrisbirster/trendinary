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

func TestOpenTursoRejectsUnsupportedSchemesBeforeNetwork(t *testing.T) {
	for _, value := range []string{
		"file:///tmp/trendinary.db",
		"postgres://example.invalid/db",
	} {
		if _, err := OpenTurso(value, "token"); err == nil || !strings.Contains(err.Error(), "unsupported Turso database URL scheme") {
			t.Fatalf("OpenTurso(%q) error = %v", value, err)
		}
	}
}

func TestOpenTursoRejectsInsecureRemoteHostsBeforeNetwork(t *testing.T) {
	for _, value := range []string{
		"http://example.turso.io",
		"ws://example.turso.io",
	} {
		if _, err := OpenTurso(value, "token"); err == nil || !strings.Contains(err.Error(), "allowed only for localhost test servers") {
			t.Fatalf("OpenTurso(%q) error = %v", value, err)
		}
	}
}

func TestLocalLibSQLHostAllowsOnlyLoopbackAndReservedTestHosts(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1", "host.docker.internal", "libsql.test"} {
		if !localLibSQLHost(host) {
			t.Fatalf("expected local host %q to be allowed", host)
		}
	}
	for _, host := range []string{"example.com", "10.0.0.4", "libsql", "libsql.example.com"} {
		if localLibSQLHost(host) {
			t.Fatalf("unexpected insecure host allowed: %q", host)
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
