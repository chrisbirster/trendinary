package newsdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
	"github.com/chrisbirster/trendinary/internal/model"
)

type fakeQuota struct {
	allow bool
	calls int
}

func (q *fakeQuota) ReserveDailyQuota(_ context.Context, source string, limit int, now time.Time) (history.DailyQuotaStatus, bool, error) {
	q.calls++
	return history.DailyQuotaStatus{Source: source, Day: now.UTC().Format("2006-01-02"), Calls: q.calls, Limit: limit, Remaining: limit - q.calls}, q.allow, nil
}
func (q *fakeQuota) DailyQuota(_ context.Context, source string, limit int, now time.Time) (history.DailyQuotaStatus, error) {
	return history.DailyQuotaStatus{Source: source, Day: now.UTC().Format("2006-01-02"), Calls: q.calls, Limit: limit, Remaining: limit - q.calls}, nil
}

type fakeRecorder struct{ values []model.Signal }
func (r *fakeRecorder) RecordSignals(_ context.Context, values []model.Signal) error {
	r.values = append(r.values, values...)
	return nil
}

func TestDiscoverUsesQuotaPaginationAndPersists(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		query := r.URL.Query()
		if query.Get("apikey") != "secret" || query.Get("size") != "10" || query.Get("language") != "en" || query.Get("removeduplicate") != "1" || query.Get("category") != "technology" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if requests == 2 && query.Get("page") != "PAGE2" {
			t.Fatalf("second request page = %q", query.Get("page"))
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = w.Write([]byte(`{"status":"success","totalResults":2,"nextPage":"PAGE2","results":[{"article_id":"a1","title":"AI story","link":"https://Example.com/ai","description":"desc","pubDate":"2026-09-01 10:00:00","creator":["Alice"],"source_name":"Example News","source_url":"https://example.com"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","totalResults":2,"results":[{"article_id":"a2","title":"Security story","link":"https://security.example/story","pubDate":"2026-09-01T11:00:00Z","source_name":"Security Example"}]}`))
	}))
	defer server.Close()

	quota := &fakeQuota{allow: true}
	recorder := &fakeRecorder{}
	source := NewWithEndpoint(server.Client(), server.URL, quota, recorder, "secret", 200, "technology")
	source.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }

	first, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d", requests)
	}
	if len(first) != 1 || len(second) != 2 {
		t.Fatalf("cache sizes = %d / %d", len(first), len(second))
	}
	if first[0].Source.Domain != "example.com" || first[0].Author != "Alice" || first[0].PublishedAt != "2026-09-01T10:00:00Z" {
		t.Fatalf("first = %+v", first[0])
	}
	if len(recorder.values) != 2 {
		t.Fatalf("persisted = %d", len(recorder.values))
	}
}

func TestDiscoverReturnsCacheWhenQuotaNotDue(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"status":"success","results":[]}`))
	}))
	defer server.Close()

	quota := &fakeQuota{allow: false}
	source := NewWithEndpoint(server.Client(), server.URL, quota, nil, "secret", 200, "technology")
	source.cached = []model.Signal{{ID: "cached", Title: "Cached"}}
	values, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requests != 0 || len(values) != 1 || values[0].ID != "cached" {
		t.Fatalf("requests=%d values=%+v", requests, values)
	}
}

func TestDiscoverWithoutAPIKeyIsDisabled(t *testing.T) {
	source := NewWithEndpoint(nil, "https://example.invalid", &fakeQuota{allow: true}, nil, "", 200, "")
	values, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 0 {
		t.Fatalf("values = %+v", values)
	}
}
