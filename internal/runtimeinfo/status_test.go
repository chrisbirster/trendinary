package runtimeinfo_test

import (
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/runtimeinfo"
)

func TestCursorOnlyBatchDoesNotManufactureEventFreshness(t *testing.T) {
	status := runtimeinfo.New(true, true)
	status.StreamConnected("https://jetstream.example")
	status.StreamBatch(42, 0, time.Time{})

	snapshot := status.Snapshot().Stream
	if !snapshot.Connected || snapshot.LastCursor != 42 {
		t.Fatalf("cursor resume status = %+v", snapshot)
	}
	if !snapshot.LastEventAt.IsZero() {
		t.Fatalf("cursor-only resume manufactured event freshness: %s", snapshot.LastEventAt)
	}
}

func TestRealBatchAdvancesEventFreshness(t *testing.T) {
	status := runtimeinfo.New(true, true)
	observed := time.Date(2026, 9, 7, 20, 0, 0, 0, time.UTC)
	status.StreamBatch(43, 128, observed)

	snapshot := status.Snapshot().Stream
	if !snapshot.LastEventAt.Equal(observed) || snapshot.LastCursor != 43 || snapshot.LastBatchSize != 128 {
		t.Fatalf("real batch status = %+v", snapshot)
	}
}
