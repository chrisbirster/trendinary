package main

import (
	"fmt"
	"os"
	"strings"

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
		return store, history.BackendTurso, err
	}
	if requireTurso {
		return nil, history.BackendTurso, fmt.Errorf("TURSO_DATABASE_URL is required when TRENDINARY_REQUIRE_TURSO=1")
	}

	path := envString("TRENDINARY_DB_PATH", "trendinary.db")
	store, err := history.Open(path)
	return store, history.BackendSQLite, err
}
