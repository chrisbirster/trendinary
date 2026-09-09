package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chrisbirster/trendinary/internal/httpapi"
	"github.com/chrisbirster/trendinary/internal/store"
)

type publicOpenAPI struct {
	Paths map[string]map[string]struct {
		Responses map[string]json.RawMessage `json:"responses"`
	} `json:"paths"`
}

func loadPublicOpenAPI(t *testing.T) publicOpenAPI {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate OpenAPI contract test source")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "api", "openapi.v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var spec publicOpenAPI
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	return spec
}

func requireDocumentedResponse(t *testing.T, spec publicOpenAPI, method, template string, status int) {
	t.Helper()
	operation, ok := spec.Paths[template][strings.ToLower(method)]
	if !ok {
		t.Fatalf("OpenAPI missing %s %s", method, template)
	}
	statusKey := http.StatusText(status)
	_ = statusKey
	code := strings.TrimSpace(strings.Split(httptest.NewRecorder().Result().Proto, " ")[0])
	_ = code
	code = func() string {
		return fmtStatus(status)
	}()
	if _, ok := operation.Responses[code]; !ok {
		t.Fatalf("OpenAPI %s %s does not document handler status %s", method, template, code)
	}
}

func fmtStatus(status int) string {
	if status < 100 || status > 999 {
		return ""
	}
	return string([]byte{byte('0' + status/100), byte('0' + (status/10)%10), byte('0' + status%10)})
}

func TestOpenAPIRepresentativeHandlerStatuses(t *testing.T) {
	spec := loadPublicOpenAPI(t)
	tests := []struct {
		name     string
		method   string
		request  string
		template string
		handler  http.Handler
		want     int
	}{
		{
			name: "health",
			method: http.MethodGet,
			request: "/api/v1/healthz",
			template: "/api/v1/healthz",
			handler: httpapi.New(store.NewMemory(), http.NotFoundHandler()),
			want: http.StatusOK,
		},
		{
			name: "readiness without durable database",
			method: http.MethodGet,
			request: "/api/v1/readyz",
			template: "/api/v1/readyz",
			handler: httpapi.New(store.NewMemory(), http.NotFoundHandler()),
			want: http.StatusServiceUnavailable,
		},
		{
			name: "trend list",
			method: http.MethodGet,
			request: "/api/v1/trends",
			template: "/api/v1/trends",
			handler: httpapi.New(store.NewDemoMemory(), http.NotFoundHandler()),
			want: http.StatusOK,
		},
		{
			name: "missing trend",
			method: http.MethodGet,
			request: "/api/v1/trends/does-not-exist",
			template: "/api/v1/trends/{slug}",
			handler: httpapi.New(store.NewMemory(), http.NotFoundHandler()),
			want: http.StatusNotFound,
		},
		{
			name: "following unavailable",
			method: http.MethodPost,
			request: "/api/v1/following/radar",
			template: "/api/v1/following/radar",
			handler: httpapi.New(store.NewMemory(), http.NotFoundHandler()),
			want: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.request, nil)
			res := httptest.NewRecorder()
			tt.handler.ServeHTTP(res, req)
			if res.Code != tt.want {
				t.Fatalf("status=%d, want=%d body=%s", res.Code, tt.want, res.Body.String())
			}
			requireDocumentedResponse(t, spec, tt.method, tt.template, res.Code)
			if res.Code != http.StatusNoContent && !strings.HasPrefix(res.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("Content-Type=%q, want application/json", res.Header().Get("Content-Type"))
			}
		})
	}
}

func TestOpenAPIHealthIdentityContract(t *testing.T) {
	handler := httpapi.New(store.NewMemory(), http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ok", "service", "api_version", "release", "commit"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("health payload missing stable field %q", key)
		}
	}
	if payload["api_version"] != "v1" {
		t.Fatalf("api_version=%v, want v1", payload["api_version"])
	}
}
