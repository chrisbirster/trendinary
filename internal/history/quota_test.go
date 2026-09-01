package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestReserveDailyQuotaPersistsAndPaces(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "quota.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	status, allowed, err := store.ReserveDailyQuota(context.Background(), "newsdata", 200, now)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || status.Calls != 1 || status.Remaining != 199 || status.NextCallAt.IsZero() {
		t.Fatalf("status = %+v allowed=%v", status, allowed)
	}
	if status.NextCallAt.Sub(now) < 3*time.Minute || status.NextCallAt.Sub(now) > 5*time.Minute {
		t.Fatalf("unexpected pacing interval: %s", status.NextCallAt.Sub(now))
	}

	second, allowed, err := store.ReserveDailyQuota(context.Background(), "newsdata", 200, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if allowed || second.Calls != 1 {
		t.Fatalf("second = %+v allowed=%v", second, allowed)
	}

	third, allowed, err := store.ReserveDailyQuota(context.Background(), "newsdata", 200, status.NextCallAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || third.Calls != 2 {
		t.Fatalf("third = %+v allowed=%v", third, allowed)
	}

	read, err := store.DailyQuota(context.Background(), "newsdata", 200, third.LastCallAt)
	if err != nil {
		t.Fatal(err)
	}
	if read.Calls != 2 || read.Remaining != 198 {
		t.Fatalf("read = %+v", read)
	}
}

func TestDailyQuotaResetsByUTCDay(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "quota.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dayOne := time.Date(2026, 9, 1, 23, 59, 0, 0, time.UTC)
	if _, allowed, err := store.ReserveDailyQuota(context.Background(), "newsdata", 1, dayOne); err != nil || !allowed {
		t.Fatalf("day one allowed=%v err=%v", allowed, err)
	}
	if _, allowed, err := store.ReserveDailyQuota(context.Background(), "newsdata", 1, dayOne.Add(2*time.Minute)); err != nil || !allowed {
		t.Fatalf("day two allowed=%v err=%v", allowed, err)
	}
}
