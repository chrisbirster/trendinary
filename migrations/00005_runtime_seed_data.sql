-- +goose Up
INSERT OR IGNORE INTO editorial_sources (
  id, name, kind, url, enabled, created_at, updated_at
)
VALUES (
  'techurls',
  'TechURLs',
  'aggregator',
  'https://techurls.com/',
  1,
  strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
  strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
);

-- +goose Down
DELETE FROM editorial_sources
WHERE id = 'techurls'
  AND NOT EXISTS (
    SELECT 1 FROM content_discoveries WHERE discovery_source_id = 'techurls'
  )
  AND NOT EXISTS (
    SELECT 1 FROM ingestion_runs WHERE source_id = 'techurls'
  );
