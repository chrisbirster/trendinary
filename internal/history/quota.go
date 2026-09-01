package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var quotaSchemaReady sync.Map

type DailyQuotaStatus struct {
	Source      string    `json:"source"`
	Day         string    `json:"day"`
	Calls       int       `json:"calls"`
	Limit       int       `json:"limit"`
	Remaining   int       `json:"remaining"`
	LastCallAt  time.Time `json:"last_call_at,omitempty"`
	NextCallAt  time.Time `json:"next_call_at,omitempty"`
}

func (s *Store) ensureQuotaSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("history store is required")
	}
	if _, ok := quotaSchemaReady.Load(s); ok {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS source_api_daily_quota (
  source TEXT NOT NULL,
  day TEXT NOT NULL,
  calls INTEGER NOT NULL DEFAULT 0,
  last_call_at TEXT,
  next_call_at TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (source, day)
);
CREATE INDEX IF NOT EXISTS idx_source_api_daily_quota_day ON source_api_daily_quota(day);
`); err != nil {
		return fmt.Errorf("create source api quota schema: %w", err)
	}
	quotaSchemaReady.Store(s, struct{}{})
	return nil
}

// ReserveDailyQuota reserves one API call before the caller performs network
// I/O. Failed requests still consume a reservation, which intentionally errs on
// the side of never exceeding a provider's daily allowance after crashes or
// retries. Calls are paced across the remaining UTC day so an unused allowance
// is gradually harvested instead of being exhausted immediately after startup.
func (s *Store) ReserveDailyQuota(ctx context.Context, source string, limit int, now time.Time) (DailyQuotaStatus, bool, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" {
		return DailyQuotaStatus{}, false, fmt.Errorf("quota source is required")
	}
	if limit <= 0 {
		return DailyQuotaStatus{}, false, fmt.Errorf("quota limit must be positive")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	if err := s.ensureQuotaSchema(ctx); err != nil {
		return DailyQuotaStatus{}, false, err
	}

	day := now.Format("2006-01-02")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DailyQuotaStatus{}, false, err
	}
	defer tx.Rollback()

	calls := 0
	var lastRaw, nextRaw sql.NullString
	err = tx.QueryRowContext(ctx, `
SELECT calls, last_call_at, next_call_at
FROM source_api_daily_quota
WHERE source = ? AND day = ?`, source, day).Scan(&calls, &lastRaw, &nextRaw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return DailyQuotaStatus{}, false, fmt.Errorf("read daily quota: %w", err)
	}

	status := DailyQuotaStatus{Source: source, Day: day, Calls: calls, Limit: limit, Remaining: max(limit-calls, 0)}
	if lastRaw.Valid {
		status.LastCallAt, _ = time.Parse(time.RFC3339Nano, lastRaw.String)
	}
	if nextRaw.Valid {
		status.NextCallAt, _ = time.Parse(time.RFC3339Nano, nextRaw.String)
	}
	if calls >= limit {
		return status, false, nil
	}
	if !status.NextCallAt.IsZero() && now.Before(status.NextCallAt) {
		return status, false, nil
	}

	calls++
	remaining := limit - calls
	nextCall := time.Time{}
	if remaining > 0 {
		nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		spacing := nextDay.Sub(now) / time.Duration(remaining)
		if spacing < time.Minute {
			spacing = time.Minute
		}
		nextCall = now.Add(spacing)
	}

	var nextValue any
	if !nextCall.IsZero() {
		nextValue = nextCall.Format(time.RFC3339Nano)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO source_api_daily_quota (source, day, calls, last_call_at, next_call_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(source, day) DO UPDATE SET
  calls=excluded.calls,
  last_call_at=excluded.last_call_at,
  next_call_at=excluded.next_call_at,
  updated_at=excluded.updated_at`,
		source,
		day,
		calls,
		now.Format(time.RFC3339Nano),
		nextValue,
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return DailyQuotaStatus{}, false, fmt.Errorf("reserve daily quota: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DailyQuotaStatus{}, false, err
	}

	return DailyQuotaStatus{
		Source:     source,
		Day:        day,
		Calls:      calls,
		Limit:      limit,
		Remaining:  remaining,
		LastCallAt: now,
		NextCallAt: nextCall,
	}, true, nil
}

func (s *Store) DailyQuota(ctx context.Context, source string, limit int, now time.Time) (DailyQuotaStatus, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" || limit <= 0 {
		return DailyQuotaStatus{}, fmt.Errorf("source and positive limit are required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	if err := s.ensureQuotaSchema(ctx); err != nil {
		return DailyQuotaStatus{}, err
	}
	status := DailyQuotaStatus{Source: source, Day: now.Format("2006-01-02"), Limit: limit, Remaining: limit}
	var lastRaw, nextRaw sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT calls, last_call_at, next_call_at FROM source_api_daily_quota WHERE source=? AND day=?`, source, status.Day).Scan(&status.Calls, &lastRaw, &nextRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return DailyQuotaStatus{}, err
	}
	status.Remaining = max(limit-status.Calls, 0)
	if lastRaw.Valid {
		status.LastCallAt, _ = time.Parse(time.RFC3339Nano, lastRaw.String)
	}
	if nextRaw.Valid {
		status.NextCallAt, _ = time.Parse(time.RFC3339Nano, nextRaw.String)
	}
	return status, nil
}
