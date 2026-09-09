//go:build integration

package history

import (
	"context"
	"testing"
	"time"
)

func TestTursoRecoverySentinelPresent(t *testing.T) {
	url, token := tursoTestCredentials(t)
	store, err := OpenTursoExisting(url, token)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("restored libSQL is not ready: %v", err)
	}

	var rows int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM signals WHERE id IN ('atlas-runtime-sentinel','atlas-runtime-script-1','atlas-runtime-script-2')`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 3 {
		t.Fatalf("restored sentinel rows=%d, want 3", rows)
	}
}
