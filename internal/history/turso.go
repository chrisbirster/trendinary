package history

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tursodatabase/libsql-client-go/libsql"
)

// OpenTurso opens Trendinary's durable SQL store against a remote Turso/libSQL
// database. Fly remains stateless; all trend history, cursors, quota state, and
// private editorial state live in Turso through the shared database/sql handle.
func OpenTurso(databaseURL, authToken string) (*Store, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	authToken = strings.TrimSpace(authToken)
	if databaseURL == "" {
		return nil, fmt.Errorf("Turso database URL is required")
	}
	if authToken == "" {
		return nil, fmt.Errorf("Turso auth token is required")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Turso database URL: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "libsql", "https", "wss":
	case "http", "ws":
		if !localLibSQLHost(parsed.Hostname()) {
			return nil, fmt.Errorf("insecure Turso URL scheme %q is allowed only for localhost test servers", parsed.Scheme)
		}
	default:
		return nil, fmt.Errorf("unsupported Turso database URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("Turso database URL host is required")
	}

	connector, err := libsql.NewConnector(databaseURL, libsql.WithAuthToken(authToken))
	if err != nil {
		return nil, fmt.Errorf("configure Turso connector: %w", err)
	}
	// Remote libSQL accepts one SQL statement per Exec request, while local
	// SQLite has historically accepted schema scripts containing many statements.
	// Wrap the connector so all stores sharing this *sql.DB keep identical
	// migration semantics without each migration needing transport-specific code.
	db := sql.OpenDB(scriptConnector{inner: connector})
	// Keep a small pool: the scanner is write-light, and avoiding a large number
	// of remote streams keeps the first production topology predictable. Startup
	// ping/migrations deliberately keep no idle connections because those calls
	// run under a short-lived context; retaining their Hrana stream after the
	// context is cancelled can hand the next caller a closed libSQL connection.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(0)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to Turso: %w", err)
	}

	store := &Store{db: db}
	markBackend(store, BackendTurso)
	if err := store.migrate(ctx); err != nil {
		storeBackends.Delete(store)
		db.Close()
		return nil, err
	}

	// Startup connections have been closed instead of pooled. Enable the normal
	// small idle pool only after all work tied to the startup context is done.
	db.SetMaxIdleConns(2)
	return store, nil
}

func localLibSQLHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	default:
		return false
	}
}
