package jetstream

import (
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
