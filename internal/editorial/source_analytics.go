package editorial

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type SourceContribution struct {
	Key                          string  `json:"key"`
	Name                         string  `json:"name"`
	Signals                      int     `json:"signals"`
	Trends                       int     `json:"trends"`
	FirstHits                    int     `json:"first_hits"`
	SoloTrends                   int     `json:"solo_trends"`
	AverageLeadToBreakingMinutes float64 `json:"average_lead_to_breaking_minutes"`
}

type SourceAnalyticsReport struct {
	Since      time.Time            `json:"since"`
	Publishers []SourceContribution `json:"publishers"`
	Channels   []SourceContribution `json:"channels"`
}

type contributionAccumulator struct {
	name         string
	signals      map[string]struct{}
	trends       map[string]struct{}
	firstHits    int
	soloTrends   int
	leadMinutes  float64
	leadSamples  int
	trendFirstAt map[string]time.Time
}

type trendSourceState struct {
	publishers map[string]struct{}
	channels   map[string]struct{}
	firstAt    time.Time
	firstPub   string
	firstChan  string
}

// SourceAnalytics measures contribution rather than popularity. It answers
// which publishers and discovery channels produced distinct trend evidence,
// which ones saw a trend first, and how much lead time they had before the
// first persisted BREAKING snapshot.
func (s *Store) SourceAnalytics(ctx context.Context, since time.Time) (SourceAnalyticsReport, error) {
	if s == nil || s.db == nil {
		return SourceAnalyticsReport{}, fmt.Errorf("editorial store is unavailable")
	}
	if err := s.ensureSourceAnalyticsMembershipSchema(ctx); err != nil {
		return SourceAnalyticsReport{}, err
	}
	if since.IsZero() {
		since = time.Now().UTC().Add(-7 * 24 * time.Hour)
	}

	// observed_at is the rolling last-seen timestamp used to decide whether a
	// membership remains active in this report window. first_observed_at is
	// immutable and is the only timestamp used for first-hit / lead-time claims.
	rows, err := s.db.QueryContext(ctx, `
SELECT sig.id,
       sig.source_name,
       COALESCE(sig.source_domain, ''),
       COALESCE(sig.discovery_channel, ''),
       m.trend_key,
       COALESCE(NULLIF(m.first_observed_at, ''), m.observed_at)
FROM trend_signal_memberships m
JOIN signals sig ON sig.id = m.signal_id
WHERE m.observed_at >= ?
ORDER BY m.observed_at`, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return SourceAnalyticsReport{}, fmt.Errorf("source analytics signals: %w", err)
	}
	defer rows.Close()

	publishers := map[string]*contributionAccumulator{}
	channels := map[string]*contributionAccumulator{}
	trends := map[string]*trendSourceState{}

	get := func(values map[string]*contributionAccumulator, key, name string) *contributionAccumulator {
		if value := values[key]; value != nil {
			return value
		}
		value := &contributionAccumulator{
			name: name, signals: map[string]struct{}{}, trends: map[string]struct{}{}, trendFirstAt: map[string]time.Time{},
		}
		values[key] = value
		return value
	}

	for rows.Next() {
		var signalID, sourceName, sourceDomain, channel, trendKey, firstObservedAt string
		if err := rows.Scan(&signalID, &sourceName, &sourceDomain, &channel, &trendKey, &firstObservedAt); err != nil {
			return SourceAnalyticsReport{}, err
		}
		publisherKey := strings.TrimSpace(strings.ToLower(sourceDomain))
		if publisherKey == "" {
			publisherKey = strings.TrimSpace(strings.ToLower(sourceName))
		}
		if publisherKey == "" {
			publisherKey = "unknown"
		}
		publisherName := strings.TrimSpace(sourceName)
		if publisherName == "" {
			publisherName = publisherKey
		}
		channelKey := strings.TrimSpace(strings.ToLower(channel))
		if channelKey == "" {
			channelKey = "unknown"
		}

		pub := get(publishers, publisherKey, publisherName)
		pub.signals[signalID] = struct{}{}
		ch := get(channels, channelKey, channelKey)
		ch.signals[signalID] = struct{}{}

		key := strings.TrimSpace(trendKey)
		if key == "" {
			continue
		}
		pub.trends[key] = struct{}{}
		ch.trends[key] = struct{}{}

		at := parseAnalyticsTime(firstObservedAt)
		if existing := pub.trendFirstAt[key]; existing.IsZero() || (!at.IsZero() && at.Before(existing)) {
			pub.trendFirstAt[key] = at
		}
		if existing := ch.trendFirstAt[key]; existing.IsZero() || (!at.IsZero() && at.Before(existing)) {
			ch.trendFirstAt[key] = at
		}

		state := trends[key]
		if state == nil {
			state = &trendSourceState{publishers: map[string]struct{}{}, channels: map[string]struct{}{}}
			trends[key] = state
		}
		state.publishers[publisherKey] = struct{}{}
		state.channels[channelKey] = struct{}{}
		if state.firstAt.IsZero() || (!at.IsZero() && at.Before(state.firstAt)) {
			state.firstAt = at
			state.firstPub = publisherKey
			state.firstChan = channelKey
		}
	}
	if err := rows.Err(); err != nil {
		return SourceAnalyticsReport{}, err
	}

	for _, state := range trends {
		if state.firstPub != "" && publishers[state.firstPub] != nil {
			publishers[state.firstPub].firstHits++
		}
		if state.firstChan != "" && channels[state.firstChan] != nil {
			channels[state.firstChan].firstHits++
		}
		if len(state.publishers) == 1 {
			for key := range state.publishers {
				if publishers[key] != nil {
					publishers[key].soloTrends++
				}
			}
		}
		if len(state.channels) == 1 {
			for key := range state.channels {
				if channels[key] != nil {
					channels[key].soloTrends++
				}
			}
		}
	}

	breaking, err := s.firstBreakingByTrend(ctx, since)
	if err != nil {
		return SourceAnalyticsReport{}, err
	}
	applyLead := func(values map[string]*contributionAccumulator) {
		for _, value := range values {
			for trendKey, firstAt := range value.trendFirstAt {
				breakAt := breaking[trendKey]
				if firstAt.IsZero() || breakAt.IsZero() || breakAt.Before(firstAt) {
					continue
				}
				value.leadMinutes += breakAt.Sub(firstAt).Minutes()
				value.leadSamples++
			}
		}
	}
	applyLead(publishers)
	applyLead(channels)

	return SourceAnalyticsReport{
		Since:      since.UTC(),
		Publishers: finalizeContributions(publishers),
		Channels:   finalizeContributions(channels),
	}, nil
}

// Keep the analytics endpoint safe even if it is called before the scanner has
// ever persisted a trend membership after deployment. This mirrors the additive
// history migration and is intentionally idempotent under rolling Fly starts.
func (s *Store) ensureSourceAnalyticsMembershipSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS trend_signal_memberships (
  trend_key TEXT NOT NULL,
  signal_id TEXT NOT NULL,
  first_observed_at TEXT NOT NULL DEFAULT '',
  observed_at TEXT NOT NULL,
  PRIMARY KEY (trend_key, signal_id)
)`); err != nil {
		return fmt.Errorf("ensure source analytics memberships: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(trend_signal_memberships)`)
	if err != nil {
		return fmt.Errorf("inspect source analytics memberships: %w", err)
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == "first_observed_at" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !found {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE trend_signal_memberships ADD COLUMN first_observed_at TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("add source analytics first observation: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE trend_signal_memberships SET first_observed_at = observed_at WHERE first_observed_at = ''`); err != nil {
		return fmt.Errorf("backfill source analytics first observation: %w", err)
	}
	return nil
}

func (s *Store) firstBreakingByTrend(ctx context.Context, since time.Time) (map[string]time.Time, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT trend_key, MIN(observed_at)
FROM trend_snapshots
WHERE lifecycle = 'BREAKING' AND observed_at >= ?
GROUP BY trend_key`, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return map[string]time.Time{}, nil
		}
		return nil, fmt.Errorf("source analytics breaking snapshots: %w", err)
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		if parsed := parseAnalyticsTime(raw); !parsed.IsZero() {
			out[key] = parsed
		}
	}
	return out, rows.Err()
}

func finalizeContributions(values map[string]*contributionAccumulator) []SourceContribution {
	out := make([]SourceContribution, 0, len(values))
	for key, value := range values {
		item := SourceContribution{
			Key: key, Name: value.name, Signals: len(value.signals), Trends: len(value.trends),
			FirstHits: value.firstHits, SoloTrends: value.soloTrends,
		}
		if value.leadSamples > 0 {
			item.AverageLeadToBreakingMinutes = value.leadMinutes / float64(value.leadSamples)
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FirstHits != out[j].FirstHits {
			return out[i].FirstHits > out[j].FirstHits
		}
		if out[i].Trends != out[j].Trends {
			return out[i].Trends > out[j].Trends
		}
		if out[i].Signals != out[j].Signals {
			return out[i].Signals > out[j].Signals
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

func parseAnalyticsTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}
