package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tursodatabase/libsql-client-go/libsql"
)

type migration struct {
	version int64
	name    string
	path    string
}

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("TURSO_DATABASE_URL"))
	authToken := strings.TrimSpace(os.Getenv("TURSO_AUTH_TOKEN"))
	migrationDir := strings.TrimSpace(os.Getenv("GOOSE_MIGRATION_DIR"))
	if migrationDir == "" {
		migrationDir = "migrations"
	}
	if databaseURL == "" {
		fatalf("TURSO_DATABASE_URL is required")
	}
	if authToken == "" {
		fatalf("TURSO_AUTH_TOKEN is required")
	}

	connector, err := libsql.NewConnector(databaseURL, libsql.WithAuthToken(authToken))
	if err != nil {
		fatalf("configure Turso connector: %v", err)
	}
	db := sql.OpenDB(connector)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fatalf("connect to Turso: %v", err)
	}

	current, err := currentVersion(ctx, db)
	if err != nil {
		fatalf("read Goose migration version: %v", err)
	}
	migrations, err := collectMigrations(migrationDir)
	if err != nil {
		fatalf("collect migrations: %v", err)
	}

	fmt.Printf("Current Goose version: %d\n", current)
	pending := 0
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		contents, err := os.ReadFile(m.path)
		if err != nil {
			fatalf("read migration %s: %v", m.name, err)
		}
		up, err := upSQL(string(contents))
		if err != nil {
			fatalf("parse migration %s: %v", m.name, err)
		}
		pending++
		fmt.Printf("\n--- pending migration %s ---\n%s", m.name, up)
		if !strings.HasSuffix(up, "\n") {
			fmt.Println()
		}
	}
	if pending == 0 {
		fmt.Println("No pending migrations.")
		return
	}
	fmt.Printf("\nPending migrations: %d\n", pending)
}

func currentVersion(ctx context.Context, db *sql.DB) (int64, error) {
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'goose_db_version'`).Scan(&exists); err != nil {
		return 0, err
	}
	if exists == 0 {
		return 0, nil
	}

	rows, err := db.QueryContext(ctx, `SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	skipped := make(map[int64]struct{})
	for rows.Next() {
		var version int64
		var applied bool
		if err := rows.Scan(&version, &applied); err != nil {
			return 0, err
		}
		if _, ok := skipped[version]; ok {
			continue
		}
		if applied {
			return version, nil
		}
		skipped[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return 0, nil
}

func collectMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	migrations := make([]migration, 0, len(entries))
	seen := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("migration filename %q must start with <version>_", entry.Name())
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("migration filename %q has invalid version", entry.Name())
		}
		if prior, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d in %q and %q", version, prior, entry.Name())
		}
		seen[version] = entry.Name()
		migrations = append(migrations, migration{version: version, name: entry.Name(), path: filepath.Join(dir, entry.Name())})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	return migrations, nil
}

func upSQL(contents string) (string, error) {
	const upMarker = "-- +goose Up"
	const downMarker = "-- +goose Down"
	up := strings.Index(contents, upMarker)
	if up < 0 {
		return "", fmt.Errorf("missing %q marker", upMarker)
	}
	body := contents[up+len(upMarker):]
	if down := strings.Index(body, downMarker); down >= 0 {
		body = body[:down]
	}
	body = strings.TrimLeft(body, "\r\n")
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("empty Up migration")
	}
	return body, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
