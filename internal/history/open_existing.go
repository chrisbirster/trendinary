package history

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tursodatabase/libsql-client-go/libsql"
	"modernc.org/sqlite"
)

// OpenExisting opens a local SQLite database without creating or changing
// schema. Atlas owns schema creation for local runtime just as it does in
// production.
func OpenExisting(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	connector, err := sqlite.NewConnector(path)
	if err != nil {
		return nil, fmt.Errorf("configure sqlite connector: %w", err)
	}
	db := sql.OpenDB(schemaManagedConnector{inner: connector})
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db}
	if err := store.configure(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	markBackend(store, BackendSQLite)
	markExternallyManaged(store)
	return store, nil
}

// OpenAdminExisting opens local SQLite with schema mutation enabled. It exists
// only for explicit administrative commands such as `trendinary db reset`.
func OpenAdminExisting(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db}
	if err := store.configure(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	markBackend(store, BackendSQLite)
	return store, nil
}

// OpenTursoExisting opens Turso/libSQL for normal application runtime. Schema
// DDL is intercepted at the connector boundary because Atlas owns all schema
// creation and upgrades before the Fly deployment starts.
func OpenTursoExisting(databaseURL, authToken string) (*Store, error) {
	return openTursoExisting(databaseURL, authToken, true)
}

// OpenTursoAdmin opens the same Turso database with schema mutation enabled.
// It is reserved for explicit administrative commands such as `db reset`.
func OpenTursoAdmin(databaseURL, authToken string) (*Store, error) {
	return openTursoExisting(databaseURL, authToken, false)
}

func openTursoExisting(databaseURL, authToken string, schemaManaged bool) (*Store, error) {
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
	if schemaManaged {
		return finishTursoOpen(sql.OpenDB(schemaManagedConnector{inner: connector}), true)
	}
	return finishTursoOpen(sql.OpenDB(connector), false)
}

func finishTursoOpen(db *sql.DB, schemaManaged bool) (*Store, error) {
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to Turso: %w", err)
	}

	store := &Store{db: db}
	markBackend(store, BackendTurso)
	if schemaManaged {
		markExternallyManaged(store)
	}
	return store, nil
}
