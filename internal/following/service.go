package following

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type TrendProvider interface {
	Trends() []model.Trend
}

type Service struct {
	store  *Store
	trends TrendProvider
	push   *PushSender
}

type Evaluation struct {
	Radars       int `json:"radars"`
	Follows      int `json:"follows"`
	Matches      int `json:"matches"`
	Alerts       int `json:"alerts"`
	PushAttempts int `json:"push_attempts"`
}

func NewService(store *Store, trends TrendProvider, push *PushSender) *Service {
	return &Service{store: store, trends: trends, push: push}
}

func (s *Service) Store() *Store { return s.store }

func trendKey(trend model.Trend) string {
	if value := strings.TrimSpace(trend.ID); value != "" {
		return value
	}
	return strings.TrimSpace(trend.Slug)
}

func uniqueSourceCount(trend model.Trend) int {
	seen := map[string]struct{}{}
	for _, source := range trend.Sources {
		identity := strings.ToLower(strings.TrimSpace(source.Domain))
		if identity == "" {
			identity = strings.ToLower(strings.TrimSpace(source.Name))
		}
		if identity != "" {
			seen[identity] = struct{}{}
		}
	}
	return len(seen)
}

func baselineFor(followID string, trend model.Trend, observedAt time.Time) Baseline {
	return Baseline{
		FollowID: followID, TrendKey: trendKey(trend), Slug: trend.Slug, Lifecycle: trend.Status,
		Score: trend.Score, Velocity: trend.Quality.Velocity, SourceBreadth: trend.Quality.SourceBreadth,
		SourceCount: uniqueSourceCount(trend), ObservedAt: observedAt.UTC().Format(time.RFC3339Nano),
	}
}

func normalizeSearch(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func tokenMatch(haystack, needle string) bool {
	haystack = normalizeSearch(haystack)
	needle = normalizeSearch(needle)
	if needle == "" || haystack == "" {
		return false
	}
	if strings.Contains(haystack, needle) {
		return true
	}
	for _, token := range strings.Fields(needle) {
		if len(token) < 3 || !strings.Contains(haystack, token) {
			return false
		}
	}
	return true
}

func matchesFollow(follow Follow, trend model.Trend) bool {
	switch follow.Kind {
	case "trend":
		value := normalizeSearch(follow.Value)
		return value == normalizeSearch(trend.Slug) || value == normalizeSearch(trend.ID)
	case "entity":
		if tokenMatch(trend.Name, follow.Value) {
			return true
		}
		for _, alias := range trend.Aliases {
			if tokenMatch(alias, follow.Value) {
				return true
			}
		}
		return false
	case "topic":
		if tokenMatch(trend.Category, follow.Value) || tokenMatch(trend.Name, follow.Value) || tokenMatch(trend.Reason, follow.Value) {
			return true
		}
		for _, alias := range trend.Aliases {
			if tokenMatch(alias, follow.Value) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func lifecycleRank(status string) int {
	switch status {
	case "EMERGING":
		return 1
	case "RISING", "RESURFACING":
		return 2
	case "BREAKING", "PEAKING":
		return 3
	default:
		return 0
	}
}

type thresholds struct {
	velocityFloor float64
	velocityDelta float64
	scoreDelta    int
	sourceDelta   int
	breadthDelta  float64
}

func thresholdsFor(sensitivity string) thresholds {
	switch normalizeSensitivity(sensitivity) {
	case "early":
		return thresholds{velocityFloor: 0.45, velocityDelta: 0.12, scoreDelta: 10, sourceDelta: 1, breadthDelta: 0.15}
	case "quiet":
		return thresholds{velocityFloor: 0.70, velocityDelta: 0.25, scoreDelta: 20, sourceDelta: 3, breadthDelta: 0.35}
	default:
		return thresholds{velocityFloor: 0.55, velocityDelta: 0.20, scoreDelta: 15, sourceDelta: 2, breadthDelta: 0.25}
	}
}

type alertCandidate struct {
	fingerprint string
	alert       Alert
}

func candidateID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return base64.RawURLEncoding.EncodeToString(sum[:18])
}

func alertFor(follow Follow, trend model.Trend, previous Baseline, kind, title, body, discriminator string, now time.Time) alertCandidate {
	key := trendKey(trend)
	fingerprint := candidateID(follow.ID, key, kind, previous.ObservedAt, discriminator)
	return alertCandidate{
		fingerprint: fingerprint,
		alert: Alert{
			ID: candidateID("alert", fingerprint), FollowID: follow.ID, TrendKey: key, Slug: trend.Slug,
			Name: trend.Name, Kind: kind, Title: title, Body: body, CreatedAt: now.UTC().Format(time.RFC3339Nano),
		},
	}
}

func detectAlerts(follow Follow, trend model.Trend, previous Baseline, preferences Preferences, now time.Time) []alertCandidate {
	out := []alertCandidate{}
	limits := thresholdsFor(preferences.Sensitivity)
	currentSources := uniqueSourceCount(trend)

	if preferences.Resurfacing && trend.Status == "RESURFACING" && previous.Lifecycle != "RESURFACING" {
		out = append(out, alertFor(follow, trend, previous, "resurfacing",
			trend.Name+" is resurfacing",
			fmt.Sprintf("Trendinary detected a new burst after cooling. Score %d.", trend.Score), trend.Status, now))
	} else if preferences.Lifecycle && lifecycleRank(trend.Status) > lifecycleRank(previous.Lifecycle) && (trend.Status == "RISING" || trend.Status == "BREAKING" || trend.Status == "PEAKING") {
		out = append(out, alertFor(follow, trend, previous, "lifecycle",
			fmt.Sprintf("%s moved to %s", trend.Name, strings.ToLower(trend.Status)),
			fmt.Sprintf("Lifecycle changed from %s to %s. Score %d.", previous.Lifecycle, trend.Status, trend.Score), trend.Status, now))
	}

	velocityDelta := trend.Quality.Velocity - previous.Velocity
	scoreDelta := trend.Score - previous.Score
	if preferences.Velocity && ((trend.Quality.Velocity >= limits.velocityFloor && velocityDelta >= limits.velocityDelta) || scoreDelta >= limits.scoreDelta) {
		out = append(out, alertFor(follow, trend, previous, "velocity",
			trend.Name+" accelerated",
			fmt.Sprintf("Velocity moved %+d points and the Trendinary Score moved %+d.", int(velocityDelta*100), scoreDelta),
			fmt.Sprintf("%.2f:%d", trend.Quality.Velocity, trend.Score), now))
	}

	sourceDelta := currentSources - previous.SourceCount
	breadthDelta := trend.Quality.SourceBreadth - previous.SourceBreadth
	if preferences.Corroboration && currentSources >= 2 && (sourceDelta >= limits.sourceDelta || breadthDelta >= limits.breadthDelta) {
		out = append(out, alertFor(follow, trend, previous, "corroboration",
			trend.Name+" crossed into more independent sources",
			fmt.Sprintf("Evidence now spans %d independent publishers; source breadth is %d%%.", currentSources, int(trend.Quality.SourceBreadth*100)),
			fmt.Sprintf("%d:%.2f", currentSources, trend.Quality.SourceBreadth), now))
	}
	return out
}

func (s *Service) Evaluate(ctx context.Context) (Evaluation, error) {
	if s == nil || s.store == nil || s.trends == nil {
		return Evaluation{}, nil
	}
	radars, err := s.store.ListRadars(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	return s.evaluateRadars(ctx, radars)
}

func (s *Service) EvaluateRadar(ctx context.Context, radarID string) (Evaluation, error) {
	if s == nil || s.store == nil || s.trends == nil {
		return Evaluation{}, nil
	}
	radars, err := s.store.ListRadars(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	for _, radar := range radars {
		if radar.ID == radarID {
			return s.evaluateRadars(ctx, []Radar{radar})
		}
	}
	return Evaluation{}, ErrRadarNotFound
}

func (s *Service) evaluateRadars(ctx context.Context, radars []Radar) (Evaluation, error) {
	var result Evaluation
	trends := s.trends.Trends()
	now := time.Now().UTC()
	result.Radars = len(radars)

	for _, radar := range radars {
		result.Follows += len(radar.Follows)
		for _, follow := range radar.Follows {
			matched := make([]model.Trend, 0)
			for _, trend := range trends {
				if matchesFollow(follow, trend) {
					matched = append(matched, trend)
				}
			}
			sort.SliceStable(matched, func(i, j int) bool { return matched[i].Score > matched[j].Score })
			for _, trend := range matched {
				result.Matches++
				key := trendKey(trend)
				if key == "" || trend.Slug == "" {
					continue
				}
				previous, exists, err := s.store.Baseline(ctx, follow.ID, key)
				if err != nil {
					return result, err
				}
				current := baselineFor(follow.ID, trend, now)
				if !exists {
					if err := s.store.PutBaseline(ctx, current); err != nil {
						return result, err
					}
					continue
				}

				for _, candidate := range detectAlerts(follow, trend, previous, radar.Preferences, now) {
					// Persist the trigger evidence before claiming the fingerprint. This is
					// safe under concurrent evaluators because every worker computes the
					// same alert ID/context for the same baseline transition.
					if err := s.store.PutAlertContext(ctx, radar.ID, candidate.alert.ID, alertContextFrom(previous, trend, now)); err != nil {
						return result, err
					}
					inserted, err := s.store.InsertAlert(ctx, radar.ID, candidate.fingerprint, candidate.alert)
					if err != nil {
						return result, err
					}
					if !inserted {
						continue
					}
					result.Alerts++
					attempts := 0
					var pushErr error
					if s.push != nil {
						attempts, pushErr = s.push.SendAlert(ctx, radar.ID, candidate.alert)
						result.PushAttempts += attempts
					}
					s.store.RecordAlertDelivery(ctx, radar.ID, candidate.alert.ID, attempts, pushErr)
				}
				if err := s.store.PutBaseline(ctx, current); err != nil {
					return result, err
				}
			}
		}
	}
	return result, nil
}
