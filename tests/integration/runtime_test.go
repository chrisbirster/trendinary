//go:build integration

package integration

import (
	"bytes"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

var (
	repoRoot   string
	binaryPath string
)

func TestMain(m *testing.M) {
	var err error
	repoRoot, err = filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tmp, err := os.MkdirTemp("", "trendinary-integration-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binaryPath = filepath.Join(tmp, "trendinary")
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/trendinary")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build integration binary: %v\n%s\n", err, output)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

type runningServer struct {
	cmd  *exec.Cmd
	logs *bytes.Buffer
	url  string
}

func TestBlankDatabaseIsRefusedUntilMigrationsAreApplied(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "blank.db")
	cmd := exec.Command(binaryPath)
	cmd.Dir = repoRoot
	cmd.Env = testEnv(dbPath, "", nil)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("blank database unexpectedly started:\n%s", output)
	}
	if !strings.Contains(string(output), "database schema is not migrated") {
		t.Fatalf("unexpected blank database error:\n%s", output)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var tables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&tables); err != nil {
		db.Close()
		t.Fatal(err)
	}
	_ = db.Close()
	if tables != 0 {
		t.Fatalf("runtime startup created %d application tables", tables)
	}

	applyMigrations(t, dbPath)
	server := startServer(t, dbPath, nil)
	assertHealthy(t, server.url)
	stopServer(t, server)
}

func TestPreparedDatabaseBootsAndRestarts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "prepared.db")
	applyMigrations(t, dbPath)
	first := startServer(t, dbPath, nil)
	assertHealthy(t, first.url)
	stopServer(t, first)

	second := startServer(t, dbPath, nil)
	assertHealthy(t, second.url)
	stopServer(t, second)
}

func TestTwentyRestartLoopDoesNotMutateSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "restart.db")
	applyMigrations(t, dbPath)
	before := schemaSQL(t, dbPath)
	for i := 0; i < 20; i++ {
		server := startServer(t, dbPath, nil)
		assertHealthy(t, server.url)
		stopServer(t, server)
	}
	after := schemaSQL(t, dbPath)
	if before != after {
		t.Fatalf("runtime startup changed schema\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestTwoProcessesShareOnePreparedDatabaseAndBothBecomeHealthy(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shared.db")
	applyMigrations(t, dbPath)
	first := startServer(t, dbPath, nil)
	defer stopServer(t, first)
	second := startServer(t, dbPath, nil)
	defer stopServer(t, second)
	assertHealthy(t, first.url)
	assertHealthy(t, second.url)
}

func TestLegacyStartupResetEnvironmentCannotDeleteData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-reset.db")
	applyMigrations(t, dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO signals(id, source_name, discovery_channel, observed_at) VALUES('survivor','fixture','fixture','2026-09-08T00:00:00Z')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	server := startServer(t, dbPath, map[string]string{"TRENDINARY_RESET_DATABASE_ID": "legacy-footgun"})
	stopServer(t, server)

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM signals WHERE id='survivor'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("legacy startup reset environment mutated data: rows=%d", rows)
	}
}

func TestDBResetDropsSchemaAndRequiresExternalMigrationBeforeBoot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cli-reset.db")
	applyMigrations(t, dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO signals(id, source_name, discovery_channel, observed_at) VALUES('disposable','fixture','fixture','2026-09-08T00:00:00Z')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binaryPath, "db", "reset")
	cmd.Dir = repoRoot
	cmd.Env = testEnv(dbPath, "", nil)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("trendinary db reset: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "database reset complete") {
		t.Fatalf("unexpected reset output: %s", output)
	}

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var applicationTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&applicationTables); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if applicationTables != 0 {
		t.Fatalf("reset left %d application tables", applicationTables)
	}

	cmd = exec.Command(binaryPath)
	cmd.Dir = repoRoot
	cmd.Env = testEnv(dbPath, "", nil)
	output, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "database schema is not migrated") {
		t.Fatalf("runtime should refuse reset database before external migration: err=%v\n%s", err, output)
	}

	applyMigrations(t, dbPath)
	server := startServer(t, dbPath, nil)
	assertHealthy(t, server.url)
	stopServer(t, server)
}

func applyMigrations(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, name := range []string{"00001_baseline.sql", "00002_following_intelligence.sql"} {
		migration, err := os.ReadFile(filepath.Join(repoRoot, "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("apply test migration %s: %v", name, err)
		}
	}
}

func schemaSQL(t *testing.T, dbPath string) string {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT type,name,COALESCE(sql,'') FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out strings.Builder
	for rows.Next() {
		var kind, name, sqlText string
		if err := rows.Scan(&kind, &name, &sqlText); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&out, "%s %s %s\n", kind, name, sqlText)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func startServer(t *testing.T, dbPath string, extra map[string]string) *runningServer {
	t.Helper()
	port := freePort(t)
	logs := &bytes.Buffer{}
	cmd := exec.Command(binaryPath)
	cmd.Dir = repoRoot
	cmd.Env = testEnv(dbPath, strconv.Itoa(port), extra)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	server := &runningServer{cmd: cmd, logs: logs, url: fmt.Sprintf("http://127.0.0.1:%d", port)}
	client := &http.Client{Timeout: 300 * time.Millisecond}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(server.url + "/api/v1/healthz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return server
			}
		}
		if server.cmd.ProcessState != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if server.cmd.ProcessState == nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	t.Fatalf("server did not become healthy at %s\n%s", server.url, logs.String())
	return nil
}

func stopServer(t *testing.T, server *runningServer) {
	t.Helper()
	if server == nil || server.cmd == nil || server.cmd.Process == nil || server.cmd.ProcessState != nil {
		return
	}
	_ = server.cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- server.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "signal: interrupt") {
			t.Fatalf("server shutdown: %v\n%s", err, server.logs.String())
		}
	case <-time.After(3 * time.Second):
		_ = server.cmd.Process.Kill()
		<-done
		t.Fatalf("server did not stop cleanly\n%s", server.logs.String())
	}
}

func assertHealthy(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(baseURL + "/api/v1/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", response.StatusCode)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func testEnv(dbPath, port string, extra map[string]string) []string {
	blocked := map[string]bool{
		"TURSO_DATABASE_URL": true,
		"TURSO_AUTH_TOKEN": true,
		"TRENDINARY_REQUIRE_TURSO": true,
		"TRENDINARY_DB_PATH": true,
		"PORT": true,
		"TRENDINARY_SCANNER_DISABLED": true,
		"TRENDINARY_JETSTREAM_DISABLED": true,
		"TRENDINARY_GITHUB_DISABLED": true,
		"TRENDINARY_RSS_DISABLED": true,
		"TRENDINARY_WIKIPEDIA_DISABLED": true,
		"TRENDINARY_GDELT_DISABLED": true,
		"TRENDINARY_YOUTUBE_DISABLED": true,
		"TRENDINARY_NEWSDATA_DISABLED": true,
		"TRENDINARY_RESET_DATABASE_ID": true,
	}
	for key := range extra {
		blocked[key] = true
	}
	env := make([]string, 0, len(os.Environ())+16)
	for _, item := range os.Environ() {
		key := item
		if index := strings.IndexByte(item, '='); index >= 0 {
			key = item[:index]
		}
		if !blocked[key] {
			env = append(env, item)
		}
	}
	env = append(env,
		"TURSO_DATABASE_URL=",
		"TURSO_AUTH_TOKEN=",
		"TRENDINARY_REQUIRE_TURSO=",
		"TRENDINARY_DB_PATH="+dbPath,
		"TRENDINARY_SCANNER_DISABLED=1",
		"TRENDINARY_JETSTREAM_DISABLED=1",
		"TRENDINARY_GITHUB_DISABLED=1",
		"TRENDINARY_RSS_DISABLED=1",
		"TRENDINARY_WIKIPEDIA_DISABLED=1",
		"TRENDINARY_GDELT_DISABLED=1",
		"TRENDINARY_YOUTUBE_DISABLED=1",
		"TRENDINARY_NEWSDATA_DISABLED=1",
	)
	if port != "" {
		env = append(env, "PORT="+port)
	}
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}
