package scanner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type scheduledFake struct {
	calls int
	err   error
}

func (f *scheduledFake) Name() string { return "fake" }
func (f *scheduledFake) Discover(context.Context) ([]model.Signal, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return []model.Signal{{ID: "one", Title: "One"}}, nil
}

func TestScheduledSourceRespectsCadenceAndCaches(t *testing.T) {
	fake := &scheduledFake{}
	source := NewScheduledSource(fake, SourceMetadata{ID: "fake", Name: "Fake", Kind: "api", Cadence: time.Hour, StartImmediately: true})
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	source.now = func() time.Time { return now }

	first, err := source.Discover(context.Background())
	if err != nil || len(first) != 1 || fake.calls != 1 {
		t.Fatalf("first=%+v err=%v calls=%d", first, err, fake.calls)
	}
	second, err := source.Discover(context.Background())
	if err != nil || len(second) != 1 || fake.calls != 1 {
		t.Fatalf("cached=%+v err=%v calls=%d", second, err, fake.calls)
	}
	status := source.SourceStatus()
	if status.Attempts != 1 || status.Successes != 1 || status.SignalsProduced != 1 || status.CachedSignals != 1 {
		t.Fatalf("unexpected first status: %+v", status)
	}

	now = now.Add(2 * time.Hour)
	_, err = source.Discover(context.Background())
	if err != nil || fake.calls != 2 {
		t.Fatalf("refresh err=%v calls=%d", err, fake.calls)
	}
	status = source.SourceStatus()
	if status.Attempts != 2 || status.Successes != 2 || status.SignalsProduced != 2 {
		t.Fatalf("unexpected refresh status: %+v", status)
	}
	if status.LastDuration < 0 || status.AverageDuration < 0 {
		t.Fatalf("invalid durations: %+v", status)
	}
}

func TestScheduledSourceBacksOffAndKeepsLastGoodBatch(t *testing.T) {
	fake := &scheduledFake{}
	source := NewScheduledSource(fake, SourceMetadata{ID: "fake", Name: "Fake", Kind: "api", Cadence: 10 * time.Minute, StartImmediately: true})
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	source.now = func() time.Time { return now }
	if _, err := source.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}

	now = now.Add(20 * time.Minute)
	fake.err = errors.New("upstream down")
	cached, err := source.Discover(context.Background())
	if err == nil || len(cached) != 1 {
		t.Fatalf("cached=%+v err=%v", cached, err)
	}
	status := source.SourceStatus()
	if status.Failures != 1 || status.LastError == "" || !status.NextRunAt.After(now) {
		t.Fatalf("status=%+v", status)
	}
	if status.Attempts != 2 || status.Successes != 1 || status.SignalsProduced != 1 || status.CachedSignals != 1 {
		t.Fatalf("failure counters=%+v", status)
	}
}

func TestScheduledSourceStillStaggersNonWarmAPI(t *testing.T) {
	fake := &scheduledFake{}
	source := NewScheduledSource(fake, SourceMetadata{ID: "other-api", Name: "Other API", Kind: "api", Cadence: 30 * time.Minute})
	status := source.SourceStatus()
	if status.NextRunAt.IsZero() || !status.NextRunAt.After(time.Now().UTC().Add(-time.Second)) {
		t.Fatalf("initial next run not staggered: %+v", status)
	}
}

func TestScheduledSourceWarmsAllRSSFeedsAtStartup(t *testing.T) {
	fake := &scheduledFake{}
	source := NewScheduledSource(fake, SourceMetadata{ID: "rss:wired", Name: "WIRED", Kind: "rss", Cadence: 30 * time.Minute})
	if status := source.SourceStatus(); !status.NextRunAt.IsZero() {
		t.Fatalf("rss next run = %s, want immediate startup poll", status.NextRunAt)
	}
	if _, err := source.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 {
		t.Fatalf("rss calls = %d, want 1", fake.calls)
	}
}

func TestScheduledSourceBootstrapsCoreTrendFeeds(t *testing.T) {
	for _, id := range []string{"google-trends-us", "wired-top", "ars-all", "abc-top", "techcrunch"} {
		fake := &scheduledFake{}
		source := NewScheduledSource(fake, SourceMetadata{ID: id, Name: id, Kind: "rss", Cadence: time.Hour})
		if status := source.SourceStatus(); !status.NextRunAt.IsZero() {
			t.Fatalf("%s next run = %s, want immediate bootstrap", id, status.NextRunAt)
		}
		if _, err := source.Discover(context.Background()); err != nil {
			t.Fatalf("%s discover: %v", id, err)
		}
		if fake.calls != 1 {
			t.Fatalf("%s calls = %d, want 1", id, fake.calls)
		}
	}
}

func TestScheduledSourceOptionalAPIsWarmAndUseAdapterSizedTimeouts(t *testing.T) {
	cases := []struct {
		id      string
		timeout time.Duration
	}{
		{id: "gdelt", timeout: 16 * time.Second},
		{id: "newsdata-api", timeout: 13 * time.Second},
		{id: "youtube-api", timeout: 11 * time.Second},
	}
	for _, tc := range cases {
		fake := &scheduledFake{}
		source := NewScheduledSource(fake, SourceMetadata{ID: tc.id, Name: tc.id, Kind: "api", Cadence: time.Hour})
		if status := source.SourceStatus(); !status.NextRunAt.IsZero() {
			t.Fatalf("%s next run = %s, want immediate startup poll", tc.id, status.NextRunAt)
		}
		if got := source.DiscoveryTimeout(); got != tc.timeout {
			t.Fatalf("%s timeout = %s, want %s", tc.id, got, tc.timeout)
		}
	}
}
