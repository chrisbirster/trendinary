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
		applied, resetErr := history.ResetDatabaseOnce(ctx, store.DB(), resetID)
		cancel()
		if resetErr != nil {
			_ = store.Close()
			return nil, history.BackendTurso, fmt.Errorf("reset Turso database %q: %w", resetID, resetErr)
		}
		if !applied {
			return store, history.BackendTurso, nil
		}

		slog.Warn("applied one-time Turso clean-slate reset", "reset_id", resetID)
		if err := store.Close(); err != nil {
			return nil, history.BackendTurso, fmt.Errorf("close reset Turso database: %w", err)
		}
		store, err = history.OpenTurso(databaseURL, authToken)
		if err != nil {
			return nil, history.BackendTurso, fmt.Errorf("reopen Turso after reset: %w", err)
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
