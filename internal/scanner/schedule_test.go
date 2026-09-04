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

	now = now.Add(2 * time.Hour)
	_, err = source.Discover(context.Background())
	if err != nil || fake.calls != 2 {
		t.Fatalf("refresh err=%v calls=%d", err, fake.calls)
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
}

func TestScheduledSourceStaggersInitialPoll(t *testing.T) {
	fake := &scheduledFake{}
	source := NewScheduledSource(fake, SourceMetadata{ID: "rss:wired", Name: "WIRED", Kind: "rss", Cadence: 30 * time.Minute})
	status := source.SourceStatus()
	if status.NextRunAt.IsZero() || !status.NextRunAt.After(time.Now().UTC().Add(-time.Second)) {
		t.Fatalf("initial next run not staggered: %+v", status)
	}
}
