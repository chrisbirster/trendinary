package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const databaseResetTable = "trendinary_database_resets"

type resetObject struct {
	kind string
	name string
}

// ResetDatabaseOnce performs an intentional clean-slate reset of Trendinary's
// shared SQL database. It drops every application-owned table and view while
// preserving a tiny reset ledger so the same reset ID is never applied twice.
//
// The operation pins one database/sql connection because SQLite/Turso PRAGMA
// state is connection-local. BEGIN IMMEDIATE makes the reset single-winner
// across rolling Fly processes, and the reset marker plus all DDL commit in one
// transaction so a crash cannot leave a partial wipe marked as complete.
// Callers that own schema migrations should reopen or remigrate their stores
// after a reset before serving traffic.
func ResetDatabaseOnce(ctx context.Context, db *sql.DB, resetID string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database is required")
	}
	resetID = strings.TrimSpace(resetID)
	if resetID == "" {
		return false, nil
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("open reset connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS trendinary_database_resets (
  reset_id TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
)`); err != nil {
		return false, fmt.Errorf("ensure database reset ledger: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return false, fmt.Errorf("disable foreign keys for reset: %w", err)
	}
	foreignKeysDisabled := true
	defer func() {
		if foreignKeysDisabled {
			_, _ = conn.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`)
		}
	}()

	// A rolling deployment can start more than one new Machine against the same
	// Turso database. BEGIN IMMEDIATE serializes reset contenders before either
	// can inspect the marker or drop an object.
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return false, fmt.Errorf("begin database reset: %w", err)
	}
	inTransaction := true
	defer func() {
		if inTransaction {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	var existing string
	err = conn.QueryRowContext(ctx, `SELECT reset_id FROM trendinary_database_resets WHERE reset_id = ?`, resetID).Scan(&existing)
	if err == nil {
		if _, rollbackErr := conn.ExecContext(ctx, `ROLLBACK`); rollbackErr != nil {
			return false, fmt.Errorf("rollback already-applied database reset: %w", rollbackErr)
		}
		inTransaction = false
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
			return false, fmt.Errorf("restore foreign keys after skipped reset: %w", err)
		}
		foreignKeysDisabled = false
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check database reset ledger: %w", err)
	}

	rows, err := conn.QueryContext(ctx, `
SELECT type, name
FROM sqlite_master
WHERE type IN ('view', 'table')
  AND name NOT LIKE 'sqlite_%'
  AND name <> ?
ORDER BY CASE type WHEN 'view' THEN 0 ELSE 1 END, name`, databaseResetTable)
	if err != nil {
		return false, fmt.Errorf("list reset objects: %w", err)
	}
	objects := make([]resetObject, 0, 64)
	for rows.Next() {
		var object resetObject
		if err := rows.Scan(&object.kind, &object.name); err != nil {
			rows.Close()
			return false, fmt.Errorf("scan reset object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, fmt.Errorf("list reset object rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return false, fmt.Errorf("close reset object rows: %w", err)
	}

	for _, object := range objects {
		kind := strings.ToUpper(strings.TrimSpace(object.kind))
		if kind != "TABLE" && kind != "VIEW" {
			continue
		}
		statement := fmt.Sprintf("DROP %s IF EXISTS %s", kind, quoteResetIdentifier(object.name))
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return false, fmt.Errorf("drop %s %q: %w", strings.ToLower(kind), object.name, err)
		}
	}

	if _, err := conn.ExecContext(ctx, `
INSERT INTO trendinary_database_resets (reset_id, applied_at)
VALUES (?, ?)`, resetID, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return false, fmt.Errorf("record database reset: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return false, fmt.Errorf("commit database reset: %w", err)
	}
	inTransaction = false

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
		return false, fmt.Errorf("restore foreign keys after reset: %w", err)
	}
	foreignKeysDisabled = false
	return true, nil
}

func quoteResetIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
