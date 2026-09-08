//go:build integration

package history

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestTursoLifecycleAgainstLocalLibSQL(t *testing.T) {
	url := os.Getenv("TRENDINARY_TEST_TURSO_URL")
	if url == "" {
		t.Skip("TRENDINARY_TEST_TURSO_URL is not configured")
	}
	token := os.Getenv("TRENDINARY_TEST_TURSO_TOKEN")
	if token == "" {
		token = "local-test-token"
	}

	store, err := OpenTurso(url, token)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := store.DB().ExecContext(ctx, `CREATE TABLE local_libsql_sentinel (id INTEGER PRIMARY KEY); INSERT INTO local_libsql_sentinel(id) VALUES (1)`); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Reset(ctx); err != nil {
		store.Close()
		t.Fatal(err)
	}
	var sentinel, signals int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'local_libsql_sentinel'`).Scan(&sentinel); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'signals'`).Scan(&signals); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if sentinel != 0 || signals != 1 {
		store.Close()
		t.Fatalf("remote reset sentinel=%d signals=%d", sentinel, signals)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenTurso(url, token)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.DB().PingContext(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestTursoTwoStoresOpenConcurrentlyAgainstLocalLibSQL(t *testing.T) {
	url := os.Getenv("TRENDINARY_TEST_TURSO_URL")
	if url == "" {
		t.Skip("TRENDINARY_TEST_TURSO_URL is not configured")
	}
	token := os.Getenv("TRENDINARY_TEST_TURSO_TOKEN")
	if token == "" {
		token = "local-test-token"
	}

	const callers = 2
	start := make(chan struct{})
	errs := make(chan error, callers)
	stores := make(chan *Store, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			store, err := OpenTurso(url, token)
			if err == nil {
				stores <- store
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	close(stores)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for store := range stores {
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
