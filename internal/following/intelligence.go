package following

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

const defaultBriefingWindow = 24 * time.Hour

type AlertContext struct {
	AlertID             string  `json:"alert_id"`
	LifecycleBefore     string  `json:"lifecycle_before"`
	LifecycleAfter      string  `json:"lifecycle_after"`
	ScoreBefore         int     `json:"score_before"`
	ScoreAfter          int     `json:"score_after"`
	VelocityBefore      float64 `json:"velocity_before"`
	VelocityAfter       float64 `json:"velocity_after"`
	SourceCountBefore   int     `json:"source_count_before"`
	SourceCountAfter    int     `json:"source_count_after"`
	SourceBreadthBefore float64 `json:"source_breadth_before"`
	SourceBreadthAfter  float64 `json:"source_breadth_after"`
	ObservedAt          string  `json:"observed_at"`
}

type AlertQuality struct {
	TotalAlerts      int     `json:"total_alerts"`
	Rated            int     `json:"rated"`
	Useful           int     `json:"useful"`
	Noise            int     `json:"noise"`
	TooLate          int     `json:"too_late"`
	UsefulRate       float64 `json:"useful_rate"`
	PushAttempts     int     `json:"push_attempts"`
	DeliveredAlerts  int     `json:"delivered_alerts"`
	FailedDeliveries int     `json:"failed_deliveries"`
}

type BriefingItem struct {
	TrendKey      string        `json:"trend_key"`
	Slug          string        `json:"slug"`
	Name          string        `json:"name"`
	LatestAt      string        `json:"latest_at"`
	AlertCount    int           `json:"alert_count"`
	Kinds         []string      `json:"kinds"`
	Changes       []string      `json:"changes"`
	LatestContext *AlertContext `json:"latest_context,omitempty"`
}

type Briefing struct {
	Since       string         `json:"since"`
	GeneratedAt string         `json:"generated_at"`
	Quiet       bool           `json:"quiet"`
	Items       []BriefingItem `json:"items"`
}

func (s *Store) ensureIntelligenceSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS following_alert_context (
			alert_id TEXT PRIMARY KEY,
			radar_id TEXT NOT NULL,
			lifecycle_before TEXT NOT NULL,
			lifecycle_after TEXT NOT NULL,
			score_before INTEGER NOT NULL,
			score_after INTEGER NOT NULL,
			velocity_before REAL NOT NULL,
			velocity_after REAL NOT NULL,
			source_count_before INTEGER NOT NULL,
			source_count_after INTEGER NOT NULL,
			source_breadth_before REAL NOT NULL,
			source_breadth_after REAL NOT NULL,
			observed_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_following_alert_context_radar ON following_alert_context(radar_id)`,
		`CREATE TABLE IF NOT EXISTS following_alert_feedback (
			alert_id TEXT PRIMARY KEY,
			radar_id TEXT NOT NULL,
			rating TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_following_alert_feedback_radar ON following_alert_feedback(radar_id)`,
		`CREATE TABLE IF NOT EXISTS following_alert_delivery (
			alert_id TEXT PRIMARY KEY,
			radar_id TEXT NOT NULL,
			push_attempts INTEGER NOT NULL,
			status TEXT NOT NULL,
			last_error TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_following_alert_delivery_radar ON following_alert_delivery(radar_id)`,
		`CREATE TABLE IF NOT EXISTS following_briefing_checkpoint (
			radar_id TEXT PRIMARY KEY,
			seen_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("following intelligence migration: %w", err)
		}
	}
	return nil
}

func alertContextFrom(previous Baseline, trend model.Trend, now time.Time) AlertContext {
	return AlertContext{
		LifecycleBefore:     previous.Lifecycle,
		LifecycleAfter:      trend.Status,
		ScoreBefore:         previous.Score,
		ScoreAfter:          trend.Score,
		VelocityBefore:      previous.Velocity,
		VelocityAfter:       trend.Quality.Velocity,
		SourceCountBefore:   previous.SourceCount,
		SourceCountAfter:    uniqueSourceCount(trend),
		SourceBreadthBefore: previous.SourceBreadth,
		SourceBreadthAfter:  trend.Quality.SourceBreadth,
		ObservedAt:          now.UTC().Format(time.RFC3339Nano),
	}
}

func (s *Store) PutAlertContext(ctx context.Context, radarID, alertID string, value AlertContext) error {
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return err
	}
	value.AlertID = alertID
	_, err := s.db.ExecContext(ctx, `INSERT INTO following_alert_context(
alert_id,radar_id,lifecycle_before,lifecycle_after,score_before,score_after,velocity_before,velocity_after,source_count_before,source_count_after,source_breadth_before,source_breadth_after,observed_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(alert_id) DO UPDATE SET
radar_id=excluded.radar_id,lifecycle_before=excluded.lifecycle_before,lifecycle_after=excluded.lifecycle_after,score_before=excluded.score_before,score_after=excluded.score_after,velocity_before=excluded.velocity_before,velocity_after=excluded.velocity_after,source_count_before=excluded.source_count_before,source_count_after=excluded.source_count_after,source_breadth_before=excluded.source_breadth_before,source_breadth_after=excluded.source_breadth_after,observed_at=excluded.observed_at`,
		alertID, radarID, value.LifecycleBefore, value.LifecycleAfter, value.ScoreBefore, value.ScoreAfter,
		value.VelocityBefore, value.VelocityAfter, value.SourceCountBefore, value.SourceCountAfter,
		value.SourceBreadthBefore, value.SourceBreadthAfter, value.ObservedAt)
	return err
}

func (s *Store) AlertContext(ctx context.Context, radarID, alertID string) (AlertContext, bool, error) {
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return AlertContext{}, false, err
	}
	var value AlertContext
	err := s.db.QueryRowContext(ctx, `SELECT alert_id,lifecycle_before,lifecycle_after,score_before,score_after,velocity_before,velocity_after,source_count_before,source_count_after,source_breadth_before,source_breadth_after,observed_at
FROM following_alert_context WHERE radar_id=? AND alert_id=?`, radarID, alertID).Scan(
		&value.AlertID, &value.LifecycleBefore, &value.LifecycleAfter, &value.ScoreBefore, &value.ScoreAfter,
		&value.VelocityBefore, &value.VelocityAfter, &value.SourceCountBefore, &value.SourceCountAfter,
		&value.SourceBreadthBefore, &value.SourceBreadthAfter, &value.ObservedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AlertContext{}, false, nil
	}
	return value, err == nil, err
}

func (s *Store) RecordAlertDelivery(ctx context.Context, radarID, alertID string, attempts int, pushErr error) {
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return
	}
	status := "no_subscription"
	lastError := ""
	if attempts > 0 && pushErr == nil {
		status = "delivered"
	} else if attempts > 0 {
		status = "failed"
		lastError = pushErr.Error()
		if len(lastError) > 500 {
			lastError = lastError[:500]
		}
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO following_alert_delivery(alert_id,radar_id,push_attempts,status,last_error,updated_at)
VALUES(?,?,?,?,?,?) ON CONFLICT(alert_id) DO UPDATE SET push_attempts=excluded.push_attempts,status=excluded.status,last_error=excluded.last_error,updated_at=excluded.updated_at`,
		alertID, radarID, attempts, status, lastError, time.Now().UTC().Format(time.RFC3339Nano))
}

func normalizeFeedback(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "useful", "noise", "too_late":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", errors.New("feedback must be useful, noise, or too_late")
	}
}

func (s *Store) SetAlertFeedback(ctx context.Context, radarID, alertID, rating string) (string, error) {
	if _, err := s.radar(ctx, radarID); err != nil {
		return "", err
	}
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return "", err
	}
	rating, err := normalizeFeedback(rating)
	if err != nil {
		return "", err
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM following_alerts WHERE radar_id=? AND id=?`, radarID, alertID).Scan(&exists); err != nil {
		return "", err
	}
	if exists == 0 {
		return "", sql.ErrNoRows
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO following_alert_feedback(alert_id,radar_id,rating,updated_at)
VALUES(?,?,?,?) ON CONFLICT(alert_id) DO UPDATE SET radar_id=excluded.radar_id,rating=excluded.rating,updated_at=excluded.updated_at`,
		alertID, radarID, rating, time.Now().UTC().Format(time.RFC3339Nano))
	return rating, err
}

func (s *Store) AlertFeedback(ctx context.Context, radarID string) (map[string]string, error) {
	if _, err := s.radar(ctx, radarID); err != nil {
		return nil, err
	}
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT alert_id,rating FROM following_alert_feedback WHERE radar_id=?`, radarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, rating string
		if err := rows.Scan(&id, &rating); err != nil {
			return nil, err
		}
		out[id] = rating
	}
	return out, rows.Err()
}

func (s *Store) Quality(ctx context.Context, radarID string) (AlertQuality, error) {
	if _, err := s.radar(ctx, radarID); err != nil {
		return AlertQuality{}, err
	}
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return AlertQuality{}, err
	}
	var out AlertQuality
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM following_alerts WHERE radar_id=?`, radarID).Scan(&out.TotalAlerts); err != nil {
		return AlertQuality{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT rating,COUNT(*) FROM following_alert_feedback WHERE radar_id=? GROUP BY rating`, radarID)
	if err != nil {
		return AlertQuality{}, err
	}
	for rows.Next() {
		var rating string
		var count int
		if err := rows.Scan(&rating, &count); err != nil {
			_ = rows.Close()
			return AlertQuality{}, err
		}
		out.Rated += count
		switch rating {
		case "useful":
			out.Useful += count
		case "noise":
			out.Noise += count
		case "too_late":
			out.TooLate += count
		}
	}
	if err := rows.Close(); err != nil {
		return AlertQuality{}, err
	}
	if out.Rated > 0 {
		out.UsefulRate = float64(out.Useful) / float64(out.Rated)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(push_attempts),0),COALESCE(SUM(CASE WHEN status='delivered' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0)
FROM following_alert_delivery WHERE radar_id=?`, radarID).Scan(&out.PushAttempts, &out.DeliveredAlerts, &out.FailedDeliveries); err != nil {
		return AlertQuality{}, err
	}
	return out, nil
}

func (s *Store) Briefing(ctx context.Context, radarID string, now time.Time) (Briefing, error) {
	if _, err := s.radar(ctx, radarID); err != nil {
		return Briefing{}, err
	}
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return Briefing{}, err
	}
	now = now.UTC()
	since := now.Add(-defaultBriefingWindow)
	var seenAt string
	err := s.db.QueryRowContext(ctx, `SELECT seen_at FROM following_briefing_checkpoint WHERE radar_id=?`, radarID).Scan(&seenAt)
	if err == nil {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, seenAt); parseErr == nil {
			since = parsed
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Briefing{}, err
	}

	alerts, err := s.Alerts(ctx, radarID, maxAlerts)
	if err != nil {
		return Briefing{}, err
	}
	items := []BriefingItem{}
	indexes := map[string]int{}
	for _, alert := range alerts {
		created, parseErr := time.Parse(time.RFC3339Nano, alert.CreatedAt)
		if parseErr != nil || !created.After(since) {
			continue
		}
		key := alert.TrendKey
		if key == "" {
			key = alert.Slug
		}
		index, ok := indexes[key]
		if !ok {
			item := BriefingItem{
				TrendKey: key, Slug: alert.Slug, Name: alert.Name, LatestAt: alert.CreatedAt,
				Kinds: []string{}, Changes: []string{},
			}
			if contextValue, found, contextErr := s.AlertContext(ctx, radarID, alert.ID); contextErr == nil && found {
				item.LatestContext = &contextValue
			}
			items = append(items, item)
			index = len(items) - 1
			indexes[key] = index
		}
		item := &items[index]
		item.AlertCount++
		if !containsString(item.Kinds, alert.Kind) {
			item.Kinds = append(item.Kinds, alert.Kind)
		}
		if alert.Body != "" && !containsString(item.Changes, alert.Body) {
			item.Changes = append(item.Changes, alert.Body)
		}
	}
	return Briefing{
		Since: since.Format(time.RFC3339Nano), GeneratedAt: now.Format(time.RFC3339Nano),
		Quiet: len(items) == 0, Items: items,
	}, nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (s *Store) MarkBriefingSeen(ctx context.Context, radarID string, now time.Time) error {
	if _, err := s.radar(ctx, radarID); err != nil {
		return err
	}
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO following_briefing_checkpoint(radar_id,seen_at) VALUES(?,?)
ON CONFLICT(radar_id) DO UPDATE SET seen_at=excluded.seen_at`, radarID, now.UTC().Format(time.RFC3339Nano))
	return err
}
