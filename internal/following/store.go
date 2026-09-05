package following

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const maxAlerts = 200

var ErrInvalidRadarKey = errors.New("invalid radar key")
var ErrRadarNotFound = errors.New("radar not found")

type Preferences struct {
	Sensitivity   string `json:"sensitivity"`
	Lifecycle     bool   `json:"lifecycle"`
	Velocity      bool   `json:"velocity"`
	Corroboration bool   `json:"corroboration"`
	Resurfacing   bool   `json:"resurfacing"`
}

type Follow struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Value       string `json:"value"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

type Baseline struct {
	FollowID      string  `json:"follow_id"`
	TrendKey      string  `json:"trend_key"`
	Slug          string  `json:"slug"`
	Lifecycle     string  `json:"lifecycle"`
	Score         int     `json:"score"`
	Velocity      float64 `json:"velocity"`
	SourceBreadth float64 `json:"source_breadth"`
	SourceCount   int     `json:"source_count"`
	ObservedAt    string  `json:"observed_at"`
}

type Alert struct {
	ID        string `json:"id"`
	FollowID  string `json:"follow_id"`
	TrendKey  string `json:"trend_key"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	Read      bool   `json:"read"`
}

type State struct {
	Preferences Preferences `json:"preferences"`
	Follows     []Follow    `json:"follows"`
	Alerts      []Alert     `json:"alerts"`
}

type Radar struct {
	ID          string
	Preferences Preferences
	Follows     []Follow
}

type PushSubscription struct {
	ID       string `json:"id,omitempty"`
	Endpoint string `json:"endpoint"`
	P256DH   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("following database is required")
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS following_radars (
			radar_id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			sensitivity TEXT NOT NULL DEFAULT 'balanced',
			lifecycle INTEGER NOT NULL DEFAULT 1,
			velocity INTEGER NOT NULL DEFAULT 1,
			corroboration INTEGER NOT NULL DEFAULT 1,
			resurfacing INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS following_follows (
			id TEXT PRIMARY KEY,
			radar_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			value TEXT NOT NULL,
			display_name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(radar_id, kind, value)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_following_follows_radar ON following_follows(radar_id)`,
		`CREATE TABLE IF NOT EXISTS following_baselines (
			follow_id TEXT NOT NULL,
			trend_key TEXT NOT NULL,
			slug TEXT NOT NULL,
			lifecycle TEXT NOT NULL,
			score INTEGER NOT NULL,
			velocity REAL NOT NULL,
			source_breadth REAL NOT NULL,
			source_count INTEGER NOT NULL,
			observed_at TEXT NOT NULL,
			PRIMARY KEY(follow_id, trend_key)
		)`,
		`CREATE TABLE IF NOT EXISTS following_alerts (
			id TEXT PRIMARY KEY,
			radar_id TEXT NOT NULL,
			follow_id TEXT NOT NULL,
			trend_key TEXT NOT NULL,
			slug TEXT NOT NULL,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			created_at TEXT NOT NULL,
			read_at TEXT,
			UNIQUE(radar_id, fingerprint)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_following_alerts_radar_created ON following_alerts(radar_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS following_push_subscriptions (
			id TEXT PRIMARY KEY,
			radar_id TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			p256dh TEXT NOT NULL,
			auth TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_success_at TEXT,
			failures INTEGER NOT NULL DEFAULT 0,
			disabled_at TEXT,
			UNIQUE(radar_id, endpoint)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_following_push_radar ON following_push_subscriptions(radar_id)`,
		`CREATE TABLE IF NOT EXISTS following_kv (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("following migration: %w", err)
		}
	}
	return nil
}

func defaultPreferences() Preferences {
	return Preferences{Sensitivity: "balanced", Lifecycle: true, Velocity: true, Corroboration: true, Resurfacing: true}
}

func normalizeSensitivity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "early", "quiet":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "balanced"
	}
}

func decodeRadarKey(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(raw) != 32 {
		return nil, ErrInvalidRadarKey
	}
	return raw, nil
}

func RadarID(token string) (string, error) {
	raw, err := decodeRadarKey(token)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func randomToken(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *Store) CreateRadar(ctx context.Context) (string, State, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", State{}, err
	}
	id, _ := RadarID(token)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	p := defaultPreferences()
	_, err = s.db.ExecContext(ctx, `INSERT INTO following_radars
(radar_id,created_at,last_seen_at,sensitivity,lifecycle,velocity,corroboration,resurfacing)
VALUES(?,?,?,?,?,?,?,?)`, id, now, now, p.Sensitivity, 1, 1, 1, 1)
	if err != nil {
		return "", State{}, err
	}
	return token, State{Preferences: p, Follows: []Follow{}, Alerts: []Alert{}}, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) radar(ctx context.Context, radarID string) (Preferences, error) {
	var p Preferences
	var lifecycle, velocity, corroboration, resurfacing int
	err := s.db.QueryRowContext(ctx, `SELECT sensitivity,lifecycle,velocity,corroboration,resurfacing FROM following_radars WHERE radar_id=?`, radarID).
		Scan(&p.Sensitivity, &lifecycle, &velocity, &corroboration, &resurfacing)
	if errors.Is(err, sql.ErrNoRows) {
		return Preferences{}, ErrRadarNotFound
	}
	if err != nil {
		return Preferences{}, err
	}
	p.Sensitivity = normalizeSensitivity(p.Sensitivity)
	p.Lifecycle = lifecycle != 0
	p.Velocity = velocity != 0
	p.Corroboration = corroboration != 0
	p.Resurfacing = resurfacing != 0
	_, _ = s.db.ExecContext(ctx, `UPDATE following_radars SET last_seen_at=? WHERE radar_id=?`, time.Now().UTC().Format(time.RFC3339Nano), radarID)
	return p, nil
}

func (s *Store) State(ctx context.Context, radarID string) (State, error) {
	p, err := s.radar(ctx, radarID)
	if err != nil {
		return State{}, err
	}
	follows, err := s.Follows(ctx, radarID)
	if err != nil {
		return State{}, err
	}
	alerts, err := s.Alerts(ctx, radarID, maxAlerts)
	if err != nil {
		return State{}, err
	}
	return State{Preferences: p, Follows: follows, Alerts: alerts}, nil
}

func normalizeKind(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trend", "topic", "entity":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", errors.New("follow kind must be trend, topic, or entity")
	}
}

func normalizeValue(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func (s *Store) AddFollow(ctx context.Context, radarID, kind, value, displayName string) (Follow, error) {
	if _, err := s.radar(ctx, radarID); err != nil {
		return Follow{}, err
	}
	kind, err := normalizeKind(kind)
	if err != nil {
		return Follow{}, err
	}
	value = normalizeValue(value)
	if value == "" || len(value) > 160 {
		return Follow{}, errors.New("follow value is required and must be at most 160 characters")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = value
	}
	if len(displayName) > 160 {
		return Follow{}, errors.New("display name must be at most 160 characters")
	}
	seed := sha256.Sum256([]byte(radarID + "\x00" + kind + "\x00" + value))
	id := base64.RawURLEncoding.EncodeToString(seed[:18])
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO following_follows(id,radar_id,kind,value,display_name,created_at)
VALUES(?,?,?,?,?,?) ON CONFLICT(radar_id,kind,value) DO UPDATE SET display_name=excluded.display_name`, id, radarID, kind, value, displayName, now)
	if err != nil {
		return Follow{}, err
	}
	var follow Follow
	err = s.db.QueryRowContext(ctx, `SELECT id,kind,value,display_name,created_at FROM following_follows WHERE radar_id=? AND kind=? AND value=?`, radarID, kind, value).
		Scan(&follow.ID, &follow.Kind, &follow.Value, &follow.DisplayName, &follow.CreatedAt)
	return follow, err
}

func (s *Store) RemoveFollow(ctx context.Context, radarID, followID string) error {
	if _, err := s.radar(ctx, radarID); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM following_follows WHERE radar_id=? AND id=?`, radarID, followID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM following_baselines WHERE follow_id=?`, followID)
	return nil
}

func (s *Store) Follows(ctx context.Context, radarID string) ([]Follow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,kind,value,display_name,created_at FROM following_follows WHERE radar_id=? ORDER BY created_at`, radarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Follow{}
	for rows.Next() {
		var item Follow
		if err := rows.Scan(&item.ID, &item.Kind, &item.Value, &item.DisplayName, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdatePreferences(ctx context.Context, radarID string, p Preferences) (Preferences, error) {
	if _, err := s.radar(ctx, radarID); err != nil {
		return Preferences{}, err
	}
	p.Sensitivity = normalizeSensitivity(p.Sensitivity)
	_, err := s.db.ExecContext(ctx, `UPDATE following_radars SET sensitivity=?,lifecycle=?,velocity=?,corroboration=?,resurfacing=?,last_seen_at=? WHERE radar_id=?`,
		p.Sensitivity, boolInt(p.Lifecycle), boolInt(p.Velocity), boolInt(p.Corroboration), boolInt(p.Resurfacing), time.Now().UTC().Format(time.RFC3339Nano), radarID)
	return p, err
}

func (s *Store) Alerts(ctx context.Context, radarID string, limit int) ([]Alert, error) {
	if limit < 1 || limit > maxAlerts {
		limit = maxAlerts
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,follow_id,trend_key,slug,name,kind,title,body,created_at,read_at IS NOT NULL
FROM following_alerts WHERE radar_id=? ORDER BY created_at DESC LIMIT ?`, radarID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Alert{}
	for rows.Next() {
		var item Alert
		if err := rows.Scan(&item.ID, &item.FollowID, &item.TrendKey, &item.Slug, &item.Name, &item.Kind, &item.Title, &item.Body, &item.CreatedAt, &item.Read); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) MarkAlertsRead(ctx context.Context, radarID string) error {
	if _, err := s.radar(ctx, radarID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE following_alerts SET read_at=COALESCE(read_at,?) WHERE radar_id=?`, time.Now().UTC().Format(time.RFC3339Nano), radarID)
	return err
}

func (s *Store) ClearAlerts(ctx context.Context, radarID string) error {
	if _, err := s.radar(ctx, radarID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM following_alerts WHERE radar_id=?`, radarID)
	return err
}

func (s *Store) ListRadars(ctx context.Context) ([]Radar, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT radar_id,sensitivity,lifecycle,velocity,corroboration,resurfacing FROM following_radars`)
	if err != nil {
		return nil, err
	}
	out := []Radar{}
	for rows.Next() {
		var radar Radar
		var lifecycle, velocity, corroboration, resurfacing int
		if err := rows.Scan(&radar.ID, &radar.Preferences.Sensitivity, &lifecycle, &velocity, &corroboration, &resurfacing); err != nil {
			_ = rows.Close()
			return nil, err
		}
		radar.Preferences.Sensitivity = normalizeSensitivity(radar.Preferences.Sensitivity)
		radar.Preferences.Lifecycle = lifecycle != 0
		radar.Preferences.Velocity = velocity != 0
		radar.Preferences.Corroboration = corroboration != 0
		radar.Preferences.Resurfacing = resurfacing != 0
		out = append(out, radar)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// Finish the outer query before reading each radar's follows. This keeps
	// evaluation correct even when the SQL pool is intentionally constrained to
	// one connection (as in SQLite tests) and avoids unnecessary nested reads in
	// production libSQL pools.
	for index := range out {
		out[index].Follows, err = s.Follows(ctx, out[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) Baseline(ctx context.Context, followID, trendKey string) (Baseline, bool, error) {
	var b Baseline
	err := s.db.QueryRowContext(ctx, `SELECT follow_id,trend_key,slug,lifecycle,score,velocity,source_breadth,source_count,observed_at
FROM following_baselines WHERE follow_id=? AND trend_key=?`, followID, trendKey).
		Scan(&b.FollowID, &b.TrendKey, &b.Slug, &b.Lifecycle, &b.Score, &b.Velocity, &b.SourceBreadth, &b.SourceCount, &b.ObservedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Baseline{}, false, nil
	}
	return b, err == nil, err
}

func (s *Store) PutBaseline(ctx context.Context, b Baseline) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO following_baselines(follow_id,trend_key,slug,lifecycle,score,velocity,source_breadth,source_count,observed_at)
VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(follow_id,trend_key) DO UPDATE SET slug=excluded.slug,lifecycle=excluded.lifecycle,score=excluded.score,velocity=excluded.velocity,source_breadth=excluded.source_breadth,source_count=excluded.source_count,observed_at=excluded.observed_at`,
		b.FollowID, b.TrendKey, b.Slug, b.Lifecycle, b.Score, b.Velocity, b.SourceBreadth, b.SourceCount, b.ObservedAt)
	return err
}

func (s *Store) InsertAlert(ctx context.Context, radarID, fingerprint string, alert Alert) (bool, error) {
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO following_alerts(id,radar_id,follow_id,trend_key,slug,name,kind,fingerprint,title,body,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)`, alert.ID, radarID, alert.FollowID, alert.TrendKey, alert.Slug, alert.Name, alert.Kind, fingerprint, alert.Title, alert.Body, alert.CreatedAt)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	if affected > 0 {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM following_alerts WHERE radar_id=? AND id NOT IN (SELECT id FROM following_alerts WHERE radar_id=? ORDER BY created_at DESC LIMIT ?)`, radarID, radarID, maxAlerts)
	}
	return affected > 0, nil
}

func subscriptionID(radarID, endpoint string) string {
	sum := sha256.Sum256([]byte(radarID + "\x00" + endpoint))
	return base64.RawURLEncoding.EncodeToString(sum[:18])
}

func (s *Store) PutPushSubscription(ctx context.Context, radarID string, sub PushSubscription) (PushSubscription, error) {
	if _, err := s.radar(ctx, radarID); err != nil {
		return PushSubscription{}, err
	}
	sub.Endpoint = strings.TrimSpace(sub.Endpoint)
	sub.P256DH = strings.TrimSpace(sub.P256DH)
	sub.Auth = strings.TrimSpace(sub.Auth)
	if !strings.HasPrefix(sub.Endpoint, "https://") || sub.P256DH == "" || sub.Auth == "" {
		return PushSubscription{}, errors.New("valid HTTPS push endpoint, p256dh, and auth are required")
	}
	sub.ID = subscriptionID(radarID, sub.Endpoint)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO following_push_subscriptions(id,radar_id,endpoint,p256dh,auth,created_at,failures,disabled_at)
VALUES(?,?,?,?,?,?,0,NULL) ON CONFLICT(radar_id,endpoint) DO UPDATE SET p256dh=excluded.p256dh,auth=excluded.auth,failures=0,disabled_at=NULL`,
		sub.ID, radarID, sub.Endpoint, sub.P256DH, sub.Auth, now)
	return sub, err
}

func (s *Store) DeletePushSubscription(ctx context.Context, radarID, endpoint string) error {
	if _, err := s.radar(ctx, radarID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM following_push_subscriptions WHERE radar_id=? AND endpoint=?`, radarID, strings.TrimSpace(endpoint))
	return err
}

func (s *Store) PushSubscriptions(ctx context.Context, radarID string) ([]PushSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,endpoint,p256dh,auth FROM following_push_subscriptions WHERE radar_id=? AND disabled_at IS NULL`, radarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PushSubscription{}
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(&sub.ID, &sub.Endpoint, &sub.P256DH, &sub.Auth); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *Store) PushSucceeded(ctx context.Context, id string) {
	_, _ = s.db.ExecContext(ctx, `UPDATE following_push_subscriptions SET last_success_at=?,failures=0 WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id)
}

func (s *Store) PushFailed(ctx context.Context, id string, gone bool) {
	if gone {
		_, _ = s.db.ExecContext(ctx, `UPDATE following_push_subscriptions SET failures=failures+1,disabled_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id)
		return
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE following_push_subscriptions SET failures=failures+1 WHERE id=?`, id)
}

func (s *Store) KV(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM following_kv WHERE key=?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Store) PutKVIfAbsent(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO following_kv(key,value,updated_at) VALUES(?,?,?)`, key, value, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}