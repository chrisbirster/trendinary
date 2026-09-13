-- +goose Up
CREATE INDEX IF NOT EXISTS idx_trend_signal_memberships_signal_id
ON trend_signal_memberships(signal_id);

-- +goose Down
DROP INDEX IF EXISTS idx_trend_signal_memberships_signal_id;
