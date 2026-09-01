package editorial

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("editorial database is required")
	}
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS editorial_sources (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  url TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  last_attempted_at TEXT,
  last_successful_at TEXT,
  latest_item_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS content_items (
  id TEXT PRIMARY KEY,
  canonical_url TEXT NOT NULL UNIQUE,
  original_url TEXT NOT NULL,
  title TEXT NOT NULL,
  publisher TEXT NOT NULL DEFAULT '',
  publisher_domain TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT 'article',
  description TEXT NOT NULL DEFAULT '',
  image_url TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  published_at TEXT,
  discovered_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  editorial_state TEXT NOT NULL DEFAULT 'inbox',
  opened_at TEXT,
  queued_at TEXT,
  consumed_at TEXT,
  saved_at TEXT,
  rejected_at TEXT,
  editorial_score INTEGER NOT NULL DEFAULT 0,
  freshness REAL NOT NULL DEFAULT 0.5,
  personal_relevance REAL NOT NULL DEFAULT 0.5,
  novelty REAL NOT NULL DEFAULT 0.5,
  source_quality REAL NOT NULL DEFAULT 0.5,
  trend_signal REAL NOT NULL DEFAULT 0.5,
  serendipity_score REAL NOT NULL DEFAULT 0,
  serendipity INTEGER NOT NULL DEFAULT 0,
  why_interesting TEXT NOT NULL DEFAULT '',
  enrichment_status TEXT NOT NULL DEFAULT 'pending',
  enrichment_error TEXT NOT NULL DEFAULT '',
  estimated_minutes INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_content_state_score ON content_items(editorial_state, editorial_score DESC, discovered_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_discovered ON content_items(discovered_at DESC);
CREATE TABLE IF NOT EXISTS content_discoveries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  content_item_id TEXT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
  discovery_source_id TEXT NOT NULL REFERENCES editorial_sources(id),
  external_source_name TEXT NOT NULL DEFAULT '',
  source_age_text TEXT NOT NULL DEFAULT '',
  discovered_at TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  UNIQUE(content_item_id, discovery_source_id, external_source_name)
);
CREATE INDEX IF NOT EXISTS idx_discoveries_item ON content_discoveries(content_item_id, discovered_at DESC);
CREATE TABLE IF NOT EXISTS content_notes (
  content_item_id TEXT PRIMARY KEY REFERENCES content_items(id) ON DELETE CASCADE,
  note TEXT NOT NULL DEFAULT '',
  worth_sharing INTEGER,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS ingestion_runs (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL REFERENCES editorial_sources(id),
  started_at TEXT NOT NULL,
  completed_at TEXT,
  status TEXT NOT NULL,
  sections_seen INTEGER NOT NULL DEFAULT 0,
  items_seen INTEGER NOT NULL DEFAULT 0,
  items_inserted INTEGER NOT NULL DEFAULT 0,
  items_updated INTEGER NOT NULL DEFAULT 0,
  duplicates INTEGER NOT NULL DEFAULT 0,
  malformed INTEGER NOT NULL DEFAULT 0,
  errors INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_ingestion_source_time ON ingestion_runs(source_id, started_at DESC);
CREATE TABLE IF NOT EXISTS newsletter_issues (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  issue_date TEXT NOT NULL DEFAULT '',
  intro TEXT NOT NULL DEFAULT '',
  question TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS newsletter_issue_items (
  id TEXT PRIMARY KEY,
  issue_id TEXT NOT NULL REFERENCES newsletter_issues(id) ON DELETE CASCADE,
  content_item_id TEXT NOT NULL REFERENCES content_items(id),
  section TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  editor_note TEXT NOT NULL DEFAULT '',
  UNIQUE(issue_id, content_item_id, section)
);
CREATE INDEX IF NOT EXISTS idx_issue_items_order ON newsletter_issue_items(issue_id, section, position);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("editorial migrate: %w", err)
	}
	return nil
}

func (s *Store) EnsureSource(ctx context.Context, source Source) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO editorial_sources (id, name, kind, url, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET name=excluded.name, kind=excluded.kind, url=excluded.url, updated_at=excluded.updated_at`,
		source.ID, source.Name, source.Kind, source.URL, boolInt(source.Enabled), now, now)
	return err
}

func (s *Store) Sources(ctx context.Context) ([]Source, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,kind,url,enabled,last_attempted_at,last_successful_at,latest_item_count,last_error,created_at,updated_at FROM editorial_sources ORDER BY name`)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []Source{}
	for rows.Next() {
		var value Source
		var enabled int
		var attempted, successful sql.NullString
		var created, updated string
		if err := rows.Scan(&value.ID,&value.Name,&value.Kind,&value.URL,&enabled,&attempted,&successful,&value.LatestItemCount,&value.LastError,&created,&updated); err != nil { return nil, err }
		value.Enabled = enabled != 0
		value.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		value.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		value.LastAttemptedAt = parseNullableTime(attempted)
		value.LastSuccessfulAt = parseNullableTime(successful)
		out = append(out, value)
	}
	return out, rows.Err()
}

func (s *Store) SetSourceEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE editorial_sources SET enabled=?, updated_at=? WHERE id=?`, boolInt(enabled), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) Source(ctx context.Context, id string) (Source, bool, error) {
	rows, err := s.Sources(ctx)
	if err != nil { return Source{}, false, err }
	for _, source := range rows { if source.ID == id { return source, true, nil } }
	return Source{}, false, nil
}

func (s *Store) StartRun(ctx context.Context, id, sourceID string, started time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO ingestion_runs (id, source_id, started_at, status) VALUES (?, ?, ?, 'running')`, id, sourceID, started.UTC().Format(time.RFC3339Nano))
	if err != nil { return err }
	_, err = s.db.ExecContext(ctx, `UPDATE editorial_sources SET last_attempted_at=?, updated_at=? WHERE id=?`, started.UTC().Format(time.RFC3339Nano), started.UTC().Format(time.RFC3339Nano), sourceID)
	return err
}

func (s *Store) FinishRun(ctx context.Context, run IngestionRun) error {
	completed := time.Now().UTC()
	if run.CompletedAt != nil { completed = run.CompletedAt.UTC() }
	_, err := s.db.ExecContext(ctx, `UPDATE ingestion_runs SET completed_at=?,status=?,sections_seen=?,items_seen=?,items_inserted=?,items_updated=?,duplicates=?,malformed=?,errors=?,error=? WHERE id=?`,
		completed.Format(time.RFC3339Nano), run.Status, run.Metrics.SectionsSeen, run.Metrics.ItemsSeen, run.Metrics.ItemsInserted, run.Metrics.ItemsUpdated, run.Metrics.Duplicates, run.Metrics.Malformed, run.Metrics.Errors, run.Error, run.ID)
	if err != nil { return err }
	if run.Status == "success" || run.Status == "partial" {
		_, err = s.db.ExecContext(ctx, `UPDATE editorial_sources SET last_successful_at=?,latest_item_count=?,last_error=?,updated_at=? WHERE id=?`, completed.Format(time.RFC3339Nano), run.Metrics.ItemsSeen, run.Error, completed.Format(time.RFC3339Nano), run.SourceID)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE editorial_sources SET last_error=?,updated_at=? WHERE id=?`, run.Error, completed.Format(time.RFC3339Nano), run.SourceID)
	}
	return err
}

func (s *Store) Runs(ctx context.Context, sourceID string, limit int) ([]IngestionRun, error) {
	if limit <= 0 { limit = 20 }
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_id,started_at,completed_at,status,sections_seen,items_seen,items_inserted,items_updated,duplicates,malformed,errors,error FROM ingestion_runs WHERE (?='' OR source_id=?) ORDER BY started_at DESC LIMIT ?`, sourceID, sourceID, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []IngestionRun{}
	for rows.Next() {
		var r IngestionRun
		var started string
		var completed sql.NullString
		if err := rows.Scan(&r.ID,&r.SourceID,&started,&completed,&r.Status,&r.Metrics.SectionsSeen,&r.Metrics.ItemsSeen,&r.Metrics.ItemsInserted,&r.Metrics.ItemsUpdated,&r.Metrics.Duplicates,&r.Metrics.Malformed,&r.Metrics.Errors,&r.Error); err != nil { return nil, err }
		r.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		r.CompletedAt = parseNullableTime(completed)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpsertContent(ctx context.Context, item ContentItem, discovery Discovery) (inserted, updated bool, err error) {
	var existingTitle, existingPublisher string
	lookupErr := s.db.QueryRowContext(ctx, `SELECT title,publisher FROM content_items WHERE canonical_url=?`, item.CanonicalURL).Scan(&existingTitle,&existingPublisher)
	inserted = errors.Is(lookupErr, sql.ErrNoRows)
	if lookupErr != nil && !inserted { return false, false, lookupErr }
	if !inserted { updated = existingTitle != item.Title || existingPublisher != item.Publisher }

	components, _ := json.Marshal(item.ScoreComponents)
	_ = components // columns remain queryable; JSON is reconstructed on reads.
	_, err = s.db.ExecContext(ctx, `
INSERT INTO content_items (id,canonical_url,original_url,title,publisher,publisher_domain,content_type,discovered_at,updated_at,editorial_score,freshness,personal_relevance,novelty,source_quality,trend_signal,serendipity_score,serendipity,why_interesting,enrichment_status,estimated_minutes)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(canonical_url) DO UPDATE SET
 original_url=excluded.original_url,title=excluded.title,publisher=excluded.publisher,publisher_domain=excluded.publisher_domain,content_type=excluded.content_type,updated_at=excluded.updated_at,
 editorial_score=excluded.editorial_score,freshness=excluded.freshness,personal_relevance=excluded.personal_relevance,novelty=excluded.novelty,source_quality=excluded.source_quality,trend_signal=excluded.trend_signal,serendipity_score=excluded.serendipity_score,serendipity=excluded.serendipity,why_interesting=excluded.why_interesting`,
		item.ID,item.CanonicalURL,item.OriginalURL,item.Title,item.Publisher,item.PublisherDomain,string(item.ContentType),item.DiscoveredAt.UTC().Format(time.RFC3339Nano),item.UpdatedAt.UTC().Format(time.RFC3339Nano),item.EditorialScore,item.ScoreComponents.Freshness,item.ScoreComponents.PersonalRelevance,item.ScoreComponents.Novelty,item.ScoreComponents.SourceQuality,item.ScoreComponents.TrendSignal,item.ScoreComponents.Serendipity,boolInt(item.Serendipity),item.WhyInteresting,string(item.EnrichmentStatus),item.EstimatedMinutes)
	if err != nil { return false, false, err }
	var contentID string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM content_items WHERE canonical_url=?`, item.CanonicalURL).Scan(&contentID); err != nil { return false, false, err }
	_, err = s.db.ExecContext(ctx, `INSERT INTO content_discoveries (content_item_id,discovery_source_id,external_source_name,source_age_text,discovered_at,metadata_json) VALUES (?,?,?,?,?,?) ON CONFLICT(content_item_id,discovery_source_id,external_source_name) DO UPDATE SET source_age_text=excluded.source_age_text,discovered_at=excluded.discovered_at,metadata_json=excluded.metadata_json`,
		contentID,discovery.DiscoverySourceID,discovery.ExternalSourceName,discovery.SourceAgeText,discovery.DiscoveredAt.UTC().Format(time.RFC3339Nano),discovery.MetadataJSON)
	return inserted, updated, err
}

func (s *Store) Content(ctx context.Context, id string) (ContentItem, bool, error) {
	row := s.db.QueryRowContext(ctx, contentSelect+` WHERE c.id=?`, id)
	value, err := scanContent(row)
	if errors.Is(err, sql.ErrNoRows) { return ContentItem{}, false, nil }
	return value, err == nil, err
}

func (s *Store) ListContent(ctx context.Context, state State, contentType ContentType, serendipityOnly bool, sortBy string, since time.Time, limit int) ([]ContentItem, error) {
	if limit <= 0 { limit = 100 }
	if limit > 500 { limit = 500 }
	order := "c.editorial_score DESC, c.discovered_at DESC"
	if sortBy == "newest" { order = "c.discovered_at DESC" }
	if sortBy == "score" { order = "c.editorial_score DESC" }
	query := contentSelect+` WHERE (?='' OR c.editorial_state=?) AND (?='' OR c.content_type=?) AND (?=0 OR c.serendipity=1) AND (?='' OR c.discovered_at>=?) ORDER BY `+order+` LIMIT ?`
	sinceText := ""
	if !since.IsZero() { sinceText = since.UTC().Format(time.RFC3339Nano) }
	rows, err := s.db.QueryContext(ctx, query, string(state), string(state), string(contentType), string(contentType), boolInt(serendipityOnly), sinceText, sinceText, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []ContentItem{}
	for rows.Next() {
		value, err := scanContent(rows)
		if err != nil { return nil, err }
		out = append(out, value)
	}
	return out, rows.Err()
}

func (s *Store) SetState(ctx context.Context, id string, state State) error {
	if !validState(state) { return fmt.Errorf("invalid editorial state %q", state) }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	column := ""
	switch state {
	case StateQueued: column = "queued_at"
	case StateConsumed: column = "consumed_at"
	case StateSaved: column = "saved_at"
	case StateRejected: column = "rejected_at"
	}
	query := `UPDATE content_items SET editorial_state=?, updated_at=?`
	args := []any{string(state), now}
	if column != "" { query += `, `+column+`=?`; args = append(args, now) }
	query += ` WHERE id=?`; args = append(args, id)
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil { return err }
	count, _ := result.RowsAffected()
	if count == 0 { return sql.ErrNoRows }
	return nil
}

func (s *Store) MarkOpened(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `UPDATE content_items SET opened_at=COALESCE(opened_at,?),updated_at=? WHERE id=?`, now, now, id)
	return err
}

func (s *Store) PutNote(ctx context.Context, note Note) (Note, error) {
	now := time.Now().UTC()
	created := now
	var existing string
	if err := s.db.QueryRowContext(ctx, `SELECT created_at FROM content_notes WHERE content_item_id=?`, note.ContentItemID).Scan(&existing); err == nil {
		if parsed, err := time.Parse(time.RFC3339Nano, existing); err == nil { created = parsed }
	}
	var worth any
	if note.WorthSharing != nil { worth = boolInt(*note.WorthSharing) }
	_, err := s.db.ExecContext(ctx, `INSERT INTO content_notes (content_item_id,note,worth_sharing,created_at,updated_at) VALUES (?,?,?,?,?) ON CONFLICT(content_item_id) DO UPDATE SET note=excluded.note,worth_sharing=excluded.worth_sharing,updated_at=excluded.updated_at`, note.ContentItemID,note.Text,worth,created.Format(time.RFC3339Nano),now.Format(time.RFC3339Nano))
	if err != nil { return Note{}, err }
	note.CreatedAt, note.UpdatedAt = created, now
	return note, nil
}

func (s *Store) UpdateEnrichment(ctx context.Context, id, canonicalURL, description, imageURL, author, publishedAt string, status EnrichmentStatus, enrichmentErr string) error {
	var published any
	if publishedAt != "" { published = publishedAt }
	_, err := s.db.ExecContext(ctx, `UPDATE content_items SET canonical_url=CASE WHEN ?='' THEN canonical_url ELSE ? END,description=?,image_url=?,author=?,published_at=?,enrichment_status=?,enrichment_error=?,updated_at=? WHERE id=?`, canonicalURL,canonicalURL,description,imageURL,author,published,string(status),enrichmentErr,time.Now().UTC().Format(time.RFC3339Nano),id)
	return err
}

const contentSelect = `
SELECT c.id,c.canonical_url,c.original_url,c.title,c.publisher,c.publisher_domain,c.content_type,c.description,c.image_url,c.author,c.published_at,c.discovered_at,c.updated_at,c.editorial_state,c.opened_at,c.queued_at,c.consumed_at,c.saved_at,c.rejected_at,c.editorial_score,c.freshness,c.personal_relevance,c.novelty,c.source_quality,c.trend_signal,c.serendipity_score,c.serendipity,c.why_interesting,c.enrichment_status,c.enrichment_error,c.estimated_minutes,
 COALESCE(d.discovery_source_id,''),COALESCE(d.external_source_name,''),COALESCE(d.source_age_text,''),
 n.note,n.worth_sharing,n.created_at,n.updated_at
FROM content_items c
LEFT JOIN content_discoveries d ON d.id=(SELECT id FROM content_discoveries WHERE content_item_id=c.id ORDER BY discovered_at DESC LIMIT 1)
LEFT JOIN content_notes n ON n.content_item_id=c.id`

type rowScanner interface { Scan(...any) error }

func scanContent(row rowScanner) (ContentItem, error) {
	var value ContentItem
	var contentType, state, enrichment string
	var published, opened, queued, consumed, saved, rejected sql.NullString
	var discovered, updated string
	var serendipity int
	var noteText, noteCreated, noteUpdated sql.NullString
	var worth sql.NullInt64
	err := row.Scan(&value.ID,&value.CanonicalURL,&value.OriginalURL,&value.Title,&value.Publisher,&value.PublisherDomain,&contentType,&value.Description,&value.ImageURL,&value.Author,&published,&discovered,&updated,&state,&opened,&queued,&consumed,&saved,&rejected,&value.EditorialScore,&value.ScoreComponents.Freshness,&value.ScoreComponents.PersonalRelevance,&value.ScoreComponents.Novelty,&value.ScoreComponents.SourceQuality,&value.ScoreComponents.TrendSignal,&value.ScoreComponents.Serendipity,&serendipity,&value.WhyInteresting,&enrichment,&value.EnrichmentError,&value.EstimatedMinutes,&value.DiscoverySource,&value.ExternalSource,&value.SourceAgeText,&noteText,&worth,&noteCreated,&noteUpdated)
	if err != nil { return ContentItem{}, err }
	value.ContentType, value.State, value.EnrichmentStatus = ContentType(contentType), State(state), EnrichmentStatus(enrichment)
	value.Serendipity = serendipity != 0
	value.DiscoveredAt, _ = time.Parse(time.RFC3339Nano, discovered)
	value.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	value.PublishedAt = parseNullableTime(published); value.OpenedAt=parseNullableTime(opened); value.QueuedAt=parseNullableTime(queued); value.ConsumedAt=parseNullableTime(consumed); value.SavedAt=parseNullableTime(saved); value.RejectedAt=parseNullableTime(rejected)
	if noteText.Valid {
		n := Note{ContentItemID:value.ID,Text:noteText.String}
		if worth.Valid { b := worth.Int64 != 0; n.WorthSharing=&b }
		if noteCreated.Valid { n.CreatedAt,_=time.Parse(time.RFC3339Nano,noteCreated.String) }
		if noteUpdated.Valid { n.UpdatedAt,_=time.Parse(time.RFC3339Nano,noteUpdated.String) }
		value.Note=&n
	}
	return value,nil
}

func validState(state State) bool {
	switch state { case StateInbox,StateQueued,StateConsumed,StateSaved,StateRejected,StateArchived: return true }
	return false
}

func boolInt(value bool) int { if value { return 1 }; return 0 }

func parseNullableTime(value sql.NullString) *time.Time {
	if !value.Valid || value.String=="" { return nil }
	parsed, err := time.Parse(time.RFC3339Nano, value.String); if err != nil { return nil }
	return &parsed
}
