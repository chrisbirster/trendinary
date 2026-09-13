//go:build integration

package history

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

func TestTursoRuntimeUsesAtlasPreparedSchemaWithoutDDL(t *testing.T) {
	url, token := tursoTestCredentials(t)
	store, err := OpenTursoExisting(url, token)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := store.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}

	// The runtime connector treats schema mutation as an intercepted no-op. This
	// keeps old defensive CREATE IF NOT EXISTS calls harmless while Goose remains
	// the only component that can change the actual production schema.
	if _, err := store.DB().ExecContext(ctx, `CREATE TABLE runtime_must_not_create (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	var created int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='runtime_must_not_create'`).Scan(&created); err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatal("normal runtime connection changed Turso schema")
	}

	if _, err := store.DB().ExecContext(ctx, `INSERT INTO signals(id, source_name, discovery_channel, observed_at) VALUES('atlas-runtime-sentinel','fixture','fixture','2026-09-08T00:00:00Z') ON CONFLICT(id) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM signals WHERE id='atlas-runtime-sentinel'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("runtime data write rows=%d", rows)
	}

	// Moving schema ownership to Goose must not regress the transport behavior
	// that existing Turso callers relied on: libSQL accepts one statement per
	// request, so the inner script connector still splits ordinary DML scripts.
	if _, err := store.DB().ExecContext(ctx, `
INSERT INTO signals(id, source_name, discovery_channel, observed_at) VALUES('atlas-runtime-script-1','fixture','fixture','2026-09-08T00:00:01Z') ON CONFLICT(id) DO NOTHING;
INSERT INTO signals(id, source_name, discovery_channel, observed_at) VALUES('atlas-runtime-script-2','fixture','fixture','2026-09-08T00:00:02Z') ON CONFLICT(id) DO NOTHING;
`); err != nil {
		t.Fatalf("runtime multi-statement DML: %v", err)
	}
	var scriptRows int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM signals WHERE id IN ('atlas-runtime-script-1','atlas-runtime-script-2')`).Scan(&scriptRows); err != nil {
		t.Fatal(err)
	}
	if scriptRows != 2 {
		t.Fatalf("runtime multi-statement rows=%d, want 2", scriptRows)
	}
}

func TestTursoBatchedSignalAndMembershipWrites(t *testing.T) {
	url, token := tursoTestCredentials(t)
	store, err := OpenTursoExisting(url, token)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := store.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}

	values := make([]model.Signal, 0, 70)
	for i := 0; i < 70; i++ {
		values = append(values, model.Signal{
			ID:               fmt.Sprintf("turso-batch-%03d", i),
			Source:           model.Source{Name: "Batch Fixture", Domain: "example.com"},
			DiscoveryChannel: "rss",
			Title:            fmt.Sprintf("Batch signal %d", i),
			Text:             "remote libSQL batched write fixture",
			Engagement:       model.Engagement{Score: i},
		})
	}
	if err := store.RecordSignalsBatch(ctx, values); err != nil {
		t.Fatalf("RecordSignalsBatch: %v", err)
	}
	if err := store.RecordTrendSignalsBatch(ctx, "turso-batch-trend", values, time.Now().UTC()); err != nil {
		t.Fatalf("RecordTrendSignalsBatch: %v", err)
	}

	var signalCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM signals WHERE id LIKE 'turso-batch-%'`).Scan(&signalCount); err != nil {
		t.Fatal(err)
	}
	if signalCount != len(values) {
		t.Fatalf("signals=%d, want %d", signalCount, len(values))
	}
	var membershipCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM trend_signal_memberships WHERE trend_key='turso-batch-trend'`).Scan(&membershipCount); err != nil {
		t.Fatal(err)
	}
	if membershipCount != len(values) {
		t.Fatalf("memberships=%d, want %d", membershipCount, len(values))
	}
}

func TestTursoTwoRuntimeStoresOpenConcurrentlyAgainstPreparedLibSQL(t *testing.T) {
	url, token := tursoTestCredentials(t)
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
			store, err := OpenTursoExisting(url, token)
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err = store.VerifySchema(ctx)
				cancel()
			}
			if err == nil {
				stores <- store
			} else if store != nil {
				_ = store.Close()
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
		// Some local libSQL server versions end a WebSocket with EOF rather than
		// a close frame. Runtime correctness is verified above; don't turn a
		// server-side close-handshake quirk into a failed schema test.
		_ = store.Close()
	}
}

func tursoTestCredentials(t *testing.T) (string, string) {
	t.Helper()
	url := os.Getenv("TRENDINARY_TEST_TURSO_URL")
	if url == "" {
		t.Skip("TRENDINARY_TEST_TURSO_URL is not configured")
	}
	token := os.Getenv("TRENDINARY_TEST_TURSO_TOKEN")
	if token == "" {
		token = "local-test-token"
	}
	return url, token
}
