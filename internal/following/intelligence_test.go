package following

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

func seededIntelligenceRadar(t *testing.T) (*Store, string, *Service) {
	t.Helper()
	store := openTestStore(t)
	ctx := context.Background()
	key, _, err := store.CreateRadar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	radarID, _ := RadarID(key)
	if _, err := store.AddFollow(ctx, radarID, "trend", "audacity-4", "Audacity 4.0"); err != nil {
		t.Fatal(err)
	}
	provider := &mutableTrends{values: []model.Trend{trendFixture("EMERGING", 30, .20, .10, 1)}}
	service := NewService(store, provider, nil)
	if _, err := service.EvaluateRadar(ctx, radarID); err != nil {
		t.Fatal(err)
	}
	provider.values = []model.Trend{trendFixture("BREAKING", 70, .80, .70, 4)}
	if _, err := service.EvaluateRadar(ctx, radarID); err != nil {
		t.Fatal(err)
	}
	return store, radarID, service
}

func TestAlertIntelligenceCapturesTriggerContext(t *testing.T) {
	store, radarID, _ := seededIntelligenceRadar(t)
	alerts, err := store.Alerts(context.Background(), radarID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 3 {
		t.Fatalf("alerts=%d, want 3", len(alerts))
	}
	contextValue, found, err := store.AlertContext(context.Background(), radarID, alerts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("missing alert trigger context")
	}
	if contextValue.ScoreBefore != 30 || contextValue.ScoreAfter != 70 {
		t.Fatalf("score transition=%d->%d", contextValue.ScoreBefore, contextValue.ScoreAfter)
	}
	if contextValue.SourceCountBefore != 1 || contextValue.SourceCountAfter != 4 {
		t.Fatalf("source transition=%d->%d", contextValue.SourceCountBefore, contextValue.SourceCountAfter)
	}
	if contextValue.LifecycleBefore != "EMERGING" || contextValue.LifecycleAfter != "BREAKING" {
		t.Fatalf("lifecycle transition=%s->%s", contextValue.LifecycleBefore, contextValue.LifecycleAfter)
	}
}

func TestAlertFeedbackAndQualityAreDurable(t *testing.T) {
	store, radarID, _ := seededIntelligenceRadar(t)
	ctx := context.Background()
	alerts, err := store.Alerts(ctx, radarID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetAlertFeedback(ctx, radarID, alerts[0].ID, "useful"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetAlertFeedback(ctx, radarID, alerts[1].ID, "too_late"); err != nil {
		t.Fatal(err)
	}
	feedback, err := store.AlertFeedback(ctx, radarID)
	if err != nil {
		t.Fatal(err)
	}
	if feedback[alerts[0].ID] != "useful" || feedback[alerts[1].ID] != "too_late" {
		t.Fatalf("feedback=%v", feedback)
	}
	quality, err := store.Quality(ctx, radarID)
	if err != nil {
		t.Fatal(err)
	}
	if quality.TotalAlerts != 3 || quality.Rated != 2 || quality.Useful != 1 || quality.TooLate != 1 || quality.UsefulRate != 0.5 {
		t.Fatalf("quality=%+v", quality)
	}
	if _, err := store.SetAlertFeedback(ctx, radarID, alerts[0].ID, "maybe"); err == nil {
		t.Fatal("invalid feedback was accepted")
	}
}

func TestBriefingGroupsChangesAndCheckpointMakesItQuiet(t *testing.T) {
	store, radarID, _ := seededIntelligenceRadar(t)
	ctx := context.Background()
	now := time.Now().UTC().Add(time.Second)
	briefing, err := store.Briefing(ctx, radarID, now)
	if err != nil {
		t.Fatal(err)
	}
	if briefing.Quiet || len(briefing.Items) != 1 {
		t.Fatalf("briefing=%+v", briefing)
	}
	item := briefing.Items[0]
	if item.AlertCount != 3 || item.Name == "" || item.LatestContext == nil {
		t.Fatalf("briefing item=%+v", item)
	}
	if len(item.Kinds) != 3 || len(item.Changes) != 3 {
		t.Fatalf("briefing item kinds=%v changes=%v", item.Kinds, item.Changes)
	}
	if err := store.MarkBriefingSeen(ctx, radarID, now); err != nil {
		t.Fatal(err)
	}
	quiet, err := store.Briefing(ctx, radarID, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !quiet.Quiet || len(quiet.Items) != 0 {
		t.Fatalf("briefing after checkpoint=%+v", quiet)
	}
}

func TestDeleteRadarRemovesIntelligenceSidecars(t *testing.T) {
	store, radarID, _ := seededIntelligenceRadar(t)
	ctx := context.Background()
	alerts, err := store.Alerts(ctx, radarID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetAlertFeedback(ctx, radarID, alerts[0].ID, "noise"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkBriefingSeen(ctx, radarID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRadar(ctx, radarID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"following_alert_context", "following_alert_feedback", "following_alert_delivery", "following_briefing_checkpoint"} {
		var count int
		if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE radar_id=?`, radarID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s rows remaining=%d", table, count)
		}
	}
}
