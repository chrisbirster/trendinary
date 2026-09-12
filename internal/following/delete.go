package following

import "context"

func (s *Store) DeleteRadar(ctx context.Context, radarID string) error {
	if _, err := s.radar(ctx, radarID); err != nil {
		return err
	}
	if err := s.ensureIntelligenceSchema(ctx); err != nil {
		return err
	}
	follows, err := s.Follows(ctx, radarID)
	if err != nil {
		return err
	}
	for _, follow := range follows {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM following_baselines WHERE follow_id=?`, follow.ID)
	}
	for _, table := range []string{
		"following_alert_context",
		"following_alert_feedback",
		"following_alert_delivery",
		"following_briefing_checkpoint",
	} {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE radar_id=?`, radarID); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM following_push_subscriptions WHERE radar_id=?`, radarID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM following_alerts WHERE radar_id=?`, radarID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM following_follows WHERE radar_id=?`, radarID); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM following_radars WHERE radar_id=?`, radarID)
	return err
}
