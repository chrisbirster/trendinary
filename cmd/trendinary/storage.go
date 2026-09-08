package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
)

func openHistory() (*history.Store, string, error) {
	databaseURL := strings.TrimSpace(os.Getenv("TURSO_DATABASE_URL"))
	authToken := strings.TrimSpace(os.Getenv("TURSO_AUTH_TOKEN"))
	requireTurso := os.Getenv("TRENDINARY_REQUIRE_TURSO") == "1"
	resetCommand := isDatabaseResetCommand()

	var (
		store   *history.Store
		backend string
		err     error
	)
	if databaseURL != "" {
		backend = history.BackendTurso
		if authToken == "" {
			return nil, backend, fmt.Errorf("TURSO_AUTH_TOKEN is required when TURSO_DATABASE_URL is configured")
		}
		if resetCommand {
			store, err = history.OpenTursoAdmin(databaseURL, authToken)
		} else {
			store, err = history.OpenTursoExisting(databaseURL, authToken)
		}
	} else {
		backend = history.BackendSQLite
		if requireTurso {
			return nil, history.BackendTurso, fmt.Errorf("TURSO_DATABASE_URL is required when TRENDINARY_REQUIRE_TURSO=1")
		}
		path := envString("TRENDINARY_DB_PATH", "trendinary.db")
		if resetCommand {
			store, err = history.OpenAdminExisting(path)
		} else {
			store, err = history.OpenExisting(path)
		}
	}
	if err != nil {
		return nil, backend, err
	}

	// The explicit reset command must be able to open a database even when the
	// schema is absent or incomplete. Every normal application command requires
	// Atlas to have made the schema ready first.
	if !resetCommand {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		verifyErr := store.VerifySchema(ctx)
		cancel()
		if verifyErr != nil {
			_ = store.Close()
			return nil, backend, verifyErr
		}
	}
	return store, backend, nil
}

func isDatabaseResetCommand() bool {
	args := os.Args[1:]
	return len(args) == 2 && args[0] == "db" && args[1] == "reset"
}
