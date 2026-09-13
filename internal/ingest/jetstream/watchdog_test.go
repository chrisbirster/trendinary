package jetstream

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/runtimeinfo"
)

func TestDefaultStaleWatchdogRecoversWithinProductionSmokeWindow(t *testing.T) {
	collector := New(nil, nil, Config{})
	if collector.config.StaleAfter != time.Minute {
		t.Fatalf("stale after = %v, want %v", collector.config.StaleAfter, time.Minute)
	}
}

func TestShouldDropResumeGapOnlyForStalePublicResume(t *testing.T) {
	now := time.Date(2026, 9, 13, 2, 18, 0, 0, time.UTC)
	stale := runtimeinfo.StreamSnapshot{LastEventAt: now.Add(-4 * time.Hour)}
	fresh := runtimeinfo.StreamSnapshot{LastEventAt: now.Add(-time.Minute)}

	if !shouldDropResumeGap("live-resume", "", stale, now) {
		t.Fatal("stale public live resume must attach at the current tip")
	}
	if !shouldDropResumeGap("live-resume", "", runtimeinfo.StreamSnapshot{}, now) {
		t.Fatal("public live resume with no observed event must attach at the current tip")
	}
	if shouldDropResumeGap("live-resume", "", fresh, now) {
		t.Fatal("fresh public live resume must preserve its durable cursor")
	}
	if shouldDropResumeGap("archive-replay", "archive-secret", stale, now) {
		t.Fatal("authenticated archive replay must retain its cursor")
	}
	if shouldDropResumeGap("live", "", stale, now) {
		t.Fatal("already-live subscriptions do not have a resume gap to drop")
	}
}

func TestPublicResumeFreshnessExpiresEvenWhileCursorAdvances(t *testing.T) {
	now := time.Date(2026, 9, 13, 6, 0, 0, 0, time.UTC)
	connectedAt := now.Add(-time.Minute)
	snapshot := runtimeinfo.StreamSnapshot{LastCursor: 9999, LastEventAt: now.Add(-4 * time.Hour)}
	if !resumeFreshnessExpired("live-resume", "", snapshot, connectedAt, now, time.Minute) {
		t.Fatal("stale public resume must expire after grace even when its cursor advances")
	}
	if resumeFreshnessExpired("archive-replay", "archive-secret", snapshot, connectedAt, now, time.Minute) {
		t.Fatal("authenticated archive replay must be allowed to catch up historical events")
	}
}

func TestRecycleStalledSubscriptionCancelsEventIteratorContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	recycleStalledSubscription(cancel, nil)
	select {
	case <-ctx.Done():
	default:
		t.Fatal("stalled subscription must cancel its Events context")
	}
}
