package history

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"
)

const resetConnectionAttempts = 20

type resetObject struct {
	kind string
	name string
}

// Reset drops every application-owned table and view. It deliberately does not
// recreate schema: Atlas owns schema creation and upgrades, and normal
// application startup never performs migrations.
func (s *Store) Reset(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is required")
	}

	// Drain idle connections before destructive DDL. This is especially
	// important for Turso/libSQL, where an idle Hrana stream may have outlived
	// the context that created it.
	s.db.SetMaxIdleConns(0)
	if err := ResetDatabase(ctx, s.db); err != nil {
		return err
	}
	if s.Backend() == BackendTurso {
		s.db.SetMaxIdleConns(2)
	} else {
		s.db.SetMaxIdleConns(1)
	}
	return nil
}

// ResetDatabase performs the destructive part of an explicit database reset.
// It serializes callers with BEGIN IMMEDIATE and retries both lock contention
// and stale remote connections. There is deliberately no reset ledger or
// startup trigger: each direct invocation means "reset now".
func ResetDatabase(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is required")
	}

	var lastErr error
	for attempt := 1; attempt <= resetConnectionAttempts; attempt++ {
		err := resetDatabaseAttempt(ctx, db)
		if err == nil {
			return nil
		}
		if !resetRetryableError(err) {
			return err
		}
		lastErr = err
		if attempt == resetConnectionAttempts {
			break
		}
		timer := time.NewTimer(time.Duration(attempt) * 50 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return fmt.Errorf("retry database reset: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf("database reset failed after %d attempts: %w", resetConnectionAttempts, lastErr)
}

func resetDatabaseAttempt(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open reset connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return fmt.Errorf("disable foreign keys for reset: %w", err)
	}
	foreignKeysDisabled := true
	defer func() {
		if foreignKeysDisabled {
			_, _ = conn.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`)
		}
	}()

	if err := beginResetTransaction(ctx, conn); err != nil {
		return err
	}
	inTransaction := true
	defer func() {
		if inTransaction {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	rows, err := conn.QueryContext(ctx, `
SELECT type, name
FROM sqlite_master
WHERE type IN ('view', 'table')
  AND name NOT LIKE 'sqlite_%'
ORDER BY CASE type WHEN 'view' THEN 0 ELSE 1 END, name`)
	if err != nil {
		return fmt.Errorf("list reset objects: %w", err)
	}
	objects := make([]resetObject, 0, 64)
	for rows.Next() {
		var object resetObject
		if err := rows.Scan(&object.kind, &object.name); err != nil {
			rows.Close()
			return fmt.Errorf("scan reset object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("list reset object rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close reset object rows: %w", err)
	}

	for _, object := range objects {
		kind := strings.ToUpper(strings.TrimSpace(object.kind))
		if kind != "TABLE" && kind != "VIEW" {
			continue
		}
		statement := fmt.Sprintf("DROP %s IF EXISTS %s", kind, quoteResetIdentifier(object.name))
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("drop %s %q: %w", strings.ToLower(kind), object.name, err)
		}
	}

	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("commit database reset: %w", err)
	}
	inTransaction = false
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
		return fmt.Errorf("restore foreign keys after reset: %w", err)
	}
	foreignKeysDisabled = false
	return nil
}

func beginResetTransaction(ctx context.Context, conn *sql.Conn) error {
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err == nil {
			return nil
		} else if !resetLockBusy(err) {
			return fmt.Errorf("begin database reset: %w", err)
		} else if time.Now().After(deadline) {
			return fmt.Errorf("begin database reset: timed out waiting for reset lock: %w", err)
		}

		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return fmt.Errorf("begin database reset: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func resetLockBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlite_busy") ||
		strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked")
}

func resetRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) || resetLockBusy(err) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "driver: bad connection") ||
		strings.Contains(message, "stream is closed")
}

func quoteResetIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
