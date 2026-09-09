from pathlib import Path


def replace_once(path: str, old: str, new: str):
    p = Path(path)
    text = p.read_text()
    if text.count(old) != 1:
        raise SystemExit(f"expected one match in {path}, found {text.count(old)}")
    p.write_text(text.replace(old, new, 1))


Path("internal/buildinfo").mkdir(parents=True, exist_ok=True)
Path("internal/buildinfo/info.go").write_text('''package buildinfo

import "strings"

const APIVersion = "v1"

// Release and Commit are populated by production builds through -ldflags.
// Local/test builds intentionally retain deterministic development values.
var (
\tRelease = "dev"
\tCommit  = "unknown"
)

type Info struct {
\tAPIVersion string `json:"api_version"`
\tRelease    string `json:"release"`
\tCommit     string `json:"commit"`
}

func Current() Info {
\treturn Normalize(Info{APIVersion: APIVersion, Release: Release, Commit: Commit})
}

func Normalize(info Info) Info {
\tinfo.APIVersion = strings.TrimSpace(info.APIVersion)
\tinfo.Release = strings.TrimSpace(info.Release)
\tinfo.Commit = strings.TrimSpace(info.Commit)
\tif info.APIVersion == "" {
\t\tinfo.APIVersion = APIVersion
\t}
\tif info.Release == "" {
\t\tinfo.Release = "dev"
\t}
\tif info.Commit == "" {
\t\tinfo.Commit = "unknown"
\t}
\treturn info
}
''')
Path("internal/buildinfo/info_test.go").write_text('''package buildinfo

import "testing"

func TestNormalizeDefaults(t *testing.T) {
\tgot := Normalize(Info{})
\tif got.APIVersion != "v1" || got.Release != "dev" || got.Commit != "unknown" {
\t\tt.Fatalf("unexpected defaults: %+v", got)
\t}
}
''')
Path("internal/history/readiness.go").write_text('''package history

import (
\t"context"
\t"fmt"
)

// Ready verifies that the durable database is reachable and that Atlas
// left the runtime-required schema in place. It is strictly read-only.
func (s *Store) Ready(ctx context.Context) error {
\tif s == nil || s.db == nil {
\t\treturn fmt.Errorf("database is unavailable")
\t}
\tif err := s.db.PingContext(ctx); err != nil {
\t\treturn fmt.Errorf("database ping: %w", err)
\t}
\tif err := s.VerifySchema(ctx); err != nil {
\t\treturn fmt.Errorf("database schema: %w", err)
\t}
\treturn nil
}
''')

path = "internal/httpapi/server.go"
replace_once(path,
    '"time"\n\n\t"github.com/chrisbirster/trendinary/internal/engine"',
    '"time"\n\n\t"github.com/chrisbirster/trendinary/internal/buildinfo"\n\t"github.com/chrisbirster/trendinary/internal/engine"')
replace_once(path,
    '\tfollowingPush *following.PushSender\n}',
    '\tfollowingPush *following.PushSender\n\tbuild         buildinfo.Info\n}')
replace_once(path,
    'func WithHistory(historical *history.Store) Option {\n\treturn func(server *Server) {\n\t\tserver.history = historical\n\t}\n}\n',
    'func WithHistory(historical *history.Store) Option {\n\treturn func(server *Server) {\n\t\tserver.history = historical\n\t}\n}\n\nfunc WithBuildInfo(info buildinfo.Info) Option {\n\treturn func(server *Server) {\n\t\tserver.build = buildinfo.Normalize(info)\n\t}\n}\n')
replace_once(path,
    '\tserver := &Server{\n\t\tstore:    s,\n\t\tfrontend: frontend,\n\t\thn:       hackernews.NewClient(nil),\n\t\tbluesky:  bluesky.NewClient(nil),\n\t}',
    '\tserver := &Server{\n\t\tstore:    s,\n\t\tfrontend: frontend,\n\t\thn:       hackernews.NewClient(nil),\n\t\tbluesky:  bluesky.NewClient(nil),\n\t\tbuild:    buildinfo.Current(),\n\t}')
replace_once(path,
    '\tmux.HandleFunc("GET /api/v1/healthz", server.health)\n\tmux.HandleFunc("GET /api/v1/health/streams", server.streamHealth)',
    '\tmux.HandleFunc("GET /api/v1/healthz", server.health)\n\tmux.HandleFunc("GET /api/v1/readyz", server.ready)\n\tmux.HandleFunc("GET /api/v1/health/streams", server.streamHealth)')
old_health = '''func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
\twriteJSON(w, http.StatusOK, map[string]any{
\t\t"ok":        true,
\t\t"service":   "trendinary",
\t\t"version":   "v1",
\t\t"time":      time.Now().UTC().Format(time.RFC3339),
\t\t"history":   s.history != nil,
\t\t"following": s.following != nil,
\t\t"web_push":  s.followingPush != nil && s.followingPush.PublicKey() != "",
\t})
}
'''
new_health = '''func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
\tpayload := s.healthIdentity()
\tpayload["ok"] = true
\tpayload["time"] = time.Now().UTC().Format(time.RFC3339)
\tpayload["history"] = s.history != nil
\tpayload["following"] = s.following != nil
\tpayload["web_push"] = s.followingPush != nil && s.followingPush.PublicKey() != ""
\tw.Header().Set("Cache-Control", "no-store")
\twriteJSON(w, http.StatusOK, payload)
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
\tpayload := s.healthIdentity()
\tpayload["time"] = time.Now().UTC().Format(time.RFC3339)
\tw.Header().Set("Cache-Control", "no-store")
\tif s.history == nil {
\t\tpayload["ok"] = false
\t\tpayload["database"] = "not_ready"
\t\twriteJSON(w, http.StatusServiceUnavailable, payload)
\t\treturn
\t}

\tctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
\tdefer cancel()
\tif err := s.history.Ready(ctx); err != nil {
\t\tpayload["ok"] = false
\t\tpayload["database"] = "not_ready"
\t\twriteJSON(w, http.StatusServiceUnavailable, payload)
\t\treturn
\t}
\tpayload["ok"] = true
\tpayload["database"] = "ready"
\twriteJSON(w, http.StatusOK, payload)
}

func (s *Server) healthIdentity() map[string]any {
\treturn map[string]any{
\t\t"service":     "trendinary",
\t\t"version":     s.build.APIVersion,
\t\t"api_version": s.build.APIVersion,
\t\t"release":     s.build.Release,
\t\t"commit":      s.build.Commit,
\t}
}
'''
replace_once(path, old_health, new_health)

path = "internal/httpapi/server_test.go"
replace_once(path,
    '"net/http/httptest"\n\t"testing"\n\t"time"',
    '"net/http/httptest"\n\t"os"\n\t"path/filepath"\n\t"strings"\n\t"testing"\n\t"time"')
replace_once(path,
    '\t"github.com/chrisbirster/trendinary/internal/engine"',
    '\t"github.com/chrisbirster/trendinary/internal/buildinfo"\n\t"github.com/chrisbirster/trendinary/internal/engine"')
replace_once(path,
    '\t"github.com/chrisbirster/trendinary/internal/httpapi"\n\t"github.com/chrisbirster/trendinary/internal/store"',
    '\t"github.com/chrisbirster/trendinary/internal/httpapi"\n\t"github.com/chrisbirster/trendinary/internal/sqlscript"\n\t"github.com/chrisbirster/trendinary/internal/store"')
p = Path(path)
p.write_text(p.read_text() + r'''

func TestHealthReportsRuntimeIdentity(t *testing.T) {
\thandler := httpapi.New(
\t\tstore.NewMemory(),
\t\thttp.NotFoundHandler(),
\t\thttpapi.WithBuildInfo(buildinfo.Info{APIVersion: "v1", Release: "v9.8.7", Commit: "abcdef123456"}),
\t)
\treq := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
\tres := httptest.NewRecorder()
\thandler.ServeHTTP(res, req)
\tvar payload struct {
\t\tOK         bool   `json:"ok"`
\t\tVersion    string `json:"version"`
\t\tAPIVersion string `json:"api_version"`
\t\tRelease    string `json:"release"`
\t\tCommit     string `json:"commit"`
\t}
\tif err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
\t\tt.Fatal(err)
\t}
\tif !payload.OK || payload.Version != "v1" || payload.APIVersion != "v1" || payload.Release != "v9.8.7" || payload.Commit != "abcdef123456" {
\t\tt.Fatalf("unexpected health identity: %+v", payload)
\t}
}

func TestReadinessHealthyDatabase(t *testing.T) {
\thistorical := openManagedHistory(t, prepareManagedDatabase(t))
\thandler := httpapi.New(
\t\tstore.NewMemory(), http.NotFoundHandler(),
\t\thttpapi.WithHistory(historical),
\t\thttpapi.WithBuildInfo(buildinfo.Info{Release: "v9.8.7", Commit: "abc123"}),
\t)
\treq := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
\tres := httptest.NewRecorder()
\thandler.ServeHTTP(res, req)
\tif res.Code != http.StatusOK {
\t\tt.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
\t}
\tvar payload struct {
\t\tOK       bool   `json:"ok"`
\t\tDatabase string `json:"database"`
\t\tRelease  string `json:"release"`
\t\tCommit   string `json:"commit"`
\t}
\tif err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
\t\tt.Fatal(err)
\t}
\tif !payload.OK || payload.Database != "ready" || payload.Release != "v9.8.7" || payload.Commit != "abc123" {
\t\tt.Fatalf("unexpected readiness payload: %+v", payload)
\t}
}

func TestReadinessRejectsUnavailableDatabaseWithoutLeakingDetails(t *testing.T) {
\thistorical := openManagedHistory(t, prepareManagedDatabase(t))
\tif err := historical.Close(); err != nil {
\t\tt.Fatal(err)
\t}
\thandler := httpapi.New(store.NewMemory(), http.NotFoundHandler(), httpapi.WithHistory(historical))
\treq := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
\tres := httptest.NewRecorder()
\thandler.ServeHTTP(res, req)
\tif res.Code != http.StatusServiceUnavailable {
\t\tt.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
\t}
\tbody := strings.ToLower(res.Body.String())
\tfor _, forbidden := range []string{"turso_auth_token", "turso_database_url", "password", "libsql://"} {
\t\tif strings.Contains(body, forbidden) {
\t\t\tt.Fatalf("readiness response leaked %q: %s", forbidden, body)
\t\t}
\t}
}

func TestReadinessRejectsStaleSchema(t *testing.T) {
\tpath := prepareManagedDatabase(t)
\tadmin, err := history.OpenAdminExisting(path)
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tif _, err := admin.DB().Exec(`DROP TABLE quality_feedback`); err != nil {
\t\t_ = admin.Close()
\t\tt.Fatal(err)
\t}
\tif err := admin.Close(); err != nil {
\t\tt.Fatal(err)
\t}
\thistorical := openManagedHistory(t, path)
\thandler := httpapi.New(store.NewMemory(), http.NotFoundHandler(), httpapi.WithHistory(historical))
\treq := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
\tres := httptest.NewRecorder()
\thandler.ServeHTTP(res, req)
\tif res.Code != http.StatusServiceUnavailable {
\t\tt.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
\t}
}

func prepareManagedDatabase(t *testing.T) string {
\tt.Helper()
\tpath := filepath.Join(t.TempDir(), "managed.db")
\tschema, err := os.ReadFile("../../schema/trendinary.sql")
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tadmin, err := history.OpenAdminExisting(path)
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tif err := sqlscript.Execute(context.Background(), admin.DB(), string(schema)); err != nil {
\t\t_ = admin.Close()
\t\tt.Fatal(err)
\t}
\tif err := admin.Close(); err != nil {
\t\tt.Fatal(err)
\t}
\treturn path
}

func openManagedHistory(t *testing.T, path string) *history.Store {
\tt.Helper()
\thistorical, err := history.OpenExisting(path)
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tt.Cleanup(func() { _ = historical.Close() })
\treturn historical
}
''')

Path("Dockerfile").write_text('''FROM node:24-alpine AS web
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build:web

FROM golang:1.27-alpine AS build
WORKDIR /src
ARG TRENDINARY_RELEASE=dev
ARG TRENDINARY_COMMIT_SHA=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/web/dist ./internal/web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \\
    -ldflags="-s -w -X github.com/chrisbirster/trendinary/internal/buildinfo.Release=${TRENDINARY_RELEASE} -X github.com/chrisbirster/trendinary/internal/buildinfo.Commit=${TRENDINARY_COMMIT_SHA}" \\
    -o /out/trendinary ./cmd/trendinary

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \\
    && addgroup -S trendinary \\
    && adduser -S -G trendinary trendinary

COPY --from=build /out/trendinary /usr/local/bin/trendinary

USER trendinary
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/trendinary"]
''')

path = "fly.toml"
replace_once(path,
    '    interval = "15s"\n    timeout = "2s"\n    grace_period = "8s"\n    method = "GET"\n    path = "/api/v1/healthz"',
    '    interval = "15s"\n    timeout = "2s"\n    grace_period = "30s"\n    method = "GET"\n    path = "/api/v1/readyz"')

path = ".github/scripts/production-smoke.sh"
replace_once(path,
    'BASE_URL="${TRENDINARY_BASE_URL:-https://trendinary.com}"\nCURL=',
    'BASE_URL="${TRENDINARY_BASE_URL:-https://trendinary.com}"\nEXPECTED_RELEASE="${TRENDINARY_EXPECTED_RELEASE:-}"\nEXPECTED_COMMIT="${TRENDINARY_EXPECTED_COMMIT:-}"\nCURL=')
replace_once(path,
    '  if ! jq -e "$filter" "$output" >/dev/null; then',
    '  if ! jq -e --arg expected_release "$EXPECTED_RELEASE" --arg expected_commit "$EXPECTED_COMMIT" "$filter" "$output" >/dev/null; then')
replace_once(path,
    'check_json "/api/v1/healthz" \'.ok == true and .service == "trendinary" and .version == "v1" and .following == true and .web_push == true\'\ncheck_runtime_health',
    'check_json "/api/v1/healthz" \'.ok == true and .service == "trendinary" and .api_version == "v1" and .release == $expected_release and .commit == $expected_commit and .following == true and .web_push == true\'\ncheck_json "/api/v1/readyz" \'.ok == true and .service == "trendinary" and .api_version == "v1" and .release == $expected_release and .commit == $expected_commit and .database == "ready"\'\ncheck_runtime_health')
