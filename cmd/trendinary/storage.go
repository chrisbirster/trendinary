package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
)

func openHistory() (*history.Store, string, error) {
	databaseURL := strings.TrimSpace(os.Getenv("TURSO_DATABASE_URL"))
	authToken := strings.TrimSpace(os.Getenv("TURSO_AUTH_TOKEN"))
	requireTurso := os.Getenv("TRENDINARY_REQUIRE_TURSO") == "1"

	if databaseURL != "" {
		if authToken == "" {
			return nil, history.BackendTurso, fmt.Errorf("TURSO_AUTH_TOKEN is required when TURSO_DATABASE_URL is configured")
		}
		store, err := history.OpenTurso(databaseURL, authToken)
		if err != nil {
			return nil, history.BackendTurso, err
		}
		resetID := strings.TrimSpace(os.Getenv("TRENDINARY_RESET_DATABASE_ID"))
		if resetID == "" {
			return store, history.BackendTurso, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		store, applied, resetErr := resetAndReopenHistory(ctx, store, resetID, func() (*history.Store, error) {
			return history.OpenTurso(databaseURL, authToken)
		})
		cancel()
		if resetErr != nil {
			return nil, history.BackendTurso, fmt.Errorf("reset Turso database %q: %w", resetID, resetErr)
		}
		if applied {
			slog.Warn("applied one-time Turso clean-slate reset", "reset_id", resetID)
		} else {
			slog.Info("observed previously applied Turso clean-slate reset; reopened schema", "reset_id", resetID)
		}
		return store, history.BackendTurso, nil
	}
	if requireTurso {
		return nil, history.BackendTurso, fmt.Errorf("TURSO_DATABASE_URL is required when TRENDINARY_REQUIRE_TURSO=1")
	}

	path := envString("TRENDINARY_DB_PATH", "trendinary.db")
	store, err := history.Open(path)
	return store, history.BackendSQLite, err
}

// resetAndReopenHistory serializes the one-time reset and then always replaces
// the caller's Store with a freshly migrated Store. Reopening is required even
// when another process won the reset race: the losing process may have opened
// and migrated its Store before the winner dropped those same tables.
func resetAndReopenHistory(
	ctx context.Context,
	store *history.Store,
	resetID string,
	reopen func() (*history.Store, error),
) (*history.Store, bool, error) {
	applied, err := history.ResetDatabaseOnce(ctx, store.DB(), resetID)
	if err != nil {
		_ = store.Close()
		return nil, false, err
	}
	if err := store.Close(); err != nil {
		return nil, applied, fmt.Errorf("close reset database: %w", err)
	}
	fresh, err := reopen()
	if err != nil {
		return nil, applied, fmt.Errorf("reopen database after reset check: %w", err)
	}
	return fresh, applied, nil
}
