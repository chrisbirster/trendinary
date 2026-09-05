package following

import (
	"context"
	"errors"
	"strings"
	"time"
)

// RebindPushSubscription gives one browser push endpoint exactly one current
// radar owner. This matters when a user imports a Radar Key on a device that
// already had Web Push enabled for a different radar: that device must stop
// receiving the old radar's notifications immediately.
func (s *Store) RebindPushSubscription(ctx context.Context, radarID string, sub PushSubscription) (PushSubscription, error) {
	if _, err := s.radar(ctx, radarID); err != nil {
		return PushSubscription{}, err
	}
	sub.Endpoint = strings.TrimSpace(sub.Endpoint)
	sub.P256DH = strings.TrimSpace(sub.P256DH)
	sub.Auth = strings.TrimSpace(sub.Auth)
	if !strings.HasPrefix(sub.Endpoint, "https://") || sub.P256DH == "" || sub.Auth == "" {
		return PushSubscription{}, errors.New("valid HTTPS push endpoint, p256dh, and auth are required")
	}

	// v0.5 introduces this table, so production starts without legacy duplicate
	// endpoints. Keep the index creation here as an idempotent safety gate for
	// local/test databases and future rolling starts.
	if _, err := s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_following_push_endpoint_unique ON following_push_subscriptions(endpoint)`); err != nil {
		return PushSubscription{}, err
	}

	sub.ID = subscriptionID(radarID, sub.Endpoint)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO following_push_subscriptions(id,radar_id,endpoint,p256dh,auth,created_at,failures,disabled_at)
VALUES(?,?,?,?,?,?,0,NULL)
ON CONFLICT(endpoint) DO UPDATE SET
 id=excluded.id,
 radar_id=excluded.radar_id,
 p256dh=excluded.p256dh,
 auth=excluded.auth,
 created_at=excluded.created_at,
 failures=0,
 disabled_at=NULL`, sub.ID, radarID, sub.Endpoint, sub.P256DH, sub.Auth, now)
	return sub, err
}
