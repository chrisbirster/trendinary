package history

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Entity struct {
	ID        string   `json:"id"`
	Slug      string   `json:"slug"`
	Name      string   `json:"name"`
	Aliases   []string `json:"aliases,omitempty"`
	Terms     []string `json:"terms,omitempty"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

var entitySchemaReady sync.Map

const entitySchema = `
CREATE TABLE IF NOT EXISTS trend_entities (
  id TEXT PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  canonical_name TEXT NOT NULL,
  first_seen TEXT NOT NULL,
  last_seen TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS trend_entity_terms (
  entity_id TEXT NOT NULL,
  term TEXT NOT NULL,
  PRIMARY KEY (entity_id, term),
  FOREIGN KEY (entity_id) REFERENCES trend_entities(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_trend_entity_terms_term ON trend_entity_terms(term);
CREATE TABLE IF NOT EXISTS trend_entity_aliases (
  entity_id TEXT NOT NULL,
  alias_key TEXT NOT NULL,
  display_alias TEXT NOT NULL,
  PRIMARY KEY (entity_id, alias_key),
  FOREIGN KEY (entity_id) REFERENCES trend_entities(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_trend_entity_aliases_key ON trend_entity_aliases(alias_key);
`

func (s *Store) ensureEntitySchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("history store is unavailable")
	}
	if _, ok := entitySchemaReady.Load(s); ok {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, entitySchema); err != nil {
		return fmt.Errorf("ensure trend entity schema: %w", err)
	}
	entitySchemaReady.Store(s, struct{}{})
	return nil
}

// ResolveEntity reconnects a current lexical cluster to a long-lived trend
// entity. Exact aliases win; otherwise canonical-term overlap is used. This
// keeps URLs/history stable while allowing the visible wording of a trend to
// evolve from scan to scan.
func (s *Store) ResolveEntity(ctx context.Context, name, clusterKey string, terms []string, observedAt time.Time) (Entity, error) {
	if err := s.ensureEntitySchema(ctx); err != nil {
		return Entity{}, err
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	terms = normalizedTerms(terms)
	aliases := normalizedAliases(name, clusterKey)

	entity, found, err := s.findEntityByAliases(ctx, aliases)
	if err != nil {
		return Entity{}, err
	}
	if !found && len(terms) > 0 {
		entity, found, err = s.findEntityByTerms(ctx, terms)
		if err != nil {
			return Entity{}, err
		}
	}
	if !found {
		entity, err = s.createEntity(ctx, name, clusterKey, observedAt)
		if err != nil {
			return Entity{}, err
		}
	}
	if err := s.touchEntity(ctx, entity.ID, name, aliases, terms, observedAt); err != nil {
		return Entity{}, err
	}
	return s.EntityByID(ctx, entity.ID)
}

func (s *Store) EntityBySlug(ctx context.Context, slug string) (Entity, bool, error) {
	if err := s.ensureEntitySchema(ctx); err != nil {
		return Entity{}, false, err
	}
	var entity Entity
	var firstSeen, lastSeen string
	err := s.db.QueryRowContext(ctx, `
SELECT id, slug, canonical_name, first_seen, last_seen
FROM trend_entities WHERE slug = ?`, strings.TrimSpace(slug)).Scan(
		&entity.ID, &entity.Slug, &entity.Name, &firstSeen, &lastSeen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Entity{}, false, nil
	}
	if err != nil {
		return Entity{}, false, err
	}
	if err := hydrateEntityTimes(&entity, firstSeen, lastSeen); err != nil {
		return Entity{}, false, err
	}
	if err := s.loadEntityMetadata(ctx, &entity); err != nil {
		return Entity{}, false, err
	}
	return entity, true, nil
}

func (s *Store) EntityByID(ctx context.Context, id string) (Entity, error) {
	if err := s.ensureEntitySchema(ctx); err != nil {
		return Entity{}, err
	}
	var entity Entity
	var firstSeen, lastSeen string
	err := s.db.QueryRowContext(ctx, `
SELECT id, slug, canonical_name, first_seen, last_seen
FROM trend_entities WHERE id = ?`, id).Scan(
		&entity.ID, &entity.Slug, &entity.Name, &firstSeen, &lastSeen,
	)
	if err != nil {
		return Entity{}, err
	}
	if err := hydrateEntityTimes(&entity, firstSeen, lastSeen); err != nil {
		return Entity{}, err
	}
	if err := s.loadEntityMetadata(ctx, &entity); err != nil {
		return Entity{}, err
	}
	return entity, nil
}

// ResolveTrendKey maps a public slug to the stable internal trend ID. Legacy
// or seeded slugs fall back to themselves so existing history remains readable.
func (s *Store) ResolveTrendKey(ctx context.Context, slug string) (string, error) {
	entity, ok, err := s.EntityBySlug(ctx, slug)
	if err != nil {
		return "", err
	}
	if ok {
		return entity.ID, nil
	}
	return slug, nil
}

func (s *Store) findEntityByAliases(ctx context.Context, aliases []string) (Entity, bool, error) {
	for _, alias := range aliases {
		var id string
		err := s.db.QueryRowContext(ctx, `
SELECT entity_id FROM trend_entity_aliases WHERE alias_key = ? ORDER BY entity_id LIMIT 1`, alias).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return Entity{}, false, err
		}
		entity, err := s.EntityByID(ctx, id)
		return entity, err == nil, err
	}
	return Entity{}, false, nil
}

func (s *Store) findEntityByTerms(ctx context.Context, terms []string) (Entity, bool, error) {
	placeholders := make([]string, len(terms))
	args := make([]any, len(terms))
	for i, term := range terms {
		placeholders[i] = "?"
		args[i] = term
	}
	query := `
SELECT entity_id, COUNT(*) AS hits
FROM trend_entity_terms
WHERE term IN (` + strings.Join(placeholders, ",") + `)
GROUP BY entity_id
ORDER BY hits DESC, entity_id
LIMIT 1`
	var id string
	var hits int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&id, &hits); errors.Is(err, sql.ErrNoRows) {
		return Entity{}, false, nil
	} else if err != nil {
		return Entity{}, false, err
	}
	minimum := 2
	if len(terms) == 1 {
		minimum = 1
	}
	if hits < minimum {
		return Entity{}, false, nil
	}
	entity, err := s.EntityByID(ctx, id)
	return entity, err == nil, err
}

func (s *Store) createEntity(ctx context.Context, name, clusterKey string, observedAt time.Time) (Entity, error) {
	id, err := newEntityID()
	if err != nil {
		return Entity{}, err
	}
	canonicalName := strings.TrimSpace(name)
	if canonicalName == "" {
		canonicalName = strings.ReplaceAll(clusterKey, "-", " ")
	}
	if canonicalName == "" {
		canonicalName = "Emerging trend"
	}
	slug := slugify(canonicalName)
	if slug == "" {
		slug = slugify(clusterKey)
	}
	if slug == "" {
		slug = "trend"
	}
	baseSlug := slug
	for attempt := 0; attempt < 5; attempt++ {
		_, err = s.db.ExecContext(ctx, `
INSERT INTO trend_entities (id, slug, canonical_name, first_seen, last_seen)
VALUES (?, ?, ?, ?, ?)`, id, slug, canonicalName, observedAt.UTC().Format(time.RFC3339Nano), observedAt.UTC().Format(time.RFC3339Nano))
		if err == nil {
			return Entity{ID: id, Slug: slug, Name: canonicalName, FirstSeen: observedAt.UTC(), LastSeen: observedAt.UTC()}, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Entity{}, err
		}
		slug = fmt.Sprintf("%s-%s", baseSlug, strings.TrimPrefix(id, "tr_")[:6])
	}
	return Entity{}, fmt.Errorf("could not allocate unique trend slug")
}

func (s *Store) touchEntity(ctx context.Context, id, currentName string, aliases, terms []string, observedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE trend_entities SET last_seen = ? WHERE id = ?`, observedAt.UTC().Format(time.RFC3339Nano), id); err != nil {
		return err
	}
	for _, term := range terms {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO trend_entity_terms (entity_id, term) VALUES (?, ?)`, id, term); err != nil {
			return err
		}
	}
	for _, alias := range aliases {
		display := alias
		if normalizedAlias(currentName) == alias && strings.TrimSpace(currentName) != "" {
			display = strings.TrimSpace(currentName)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO trend_entity_aliases (entity_id, alias_key, display_alias) VALUES (?, ?, ?)`, id, alias, display); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) loadEntityMetadata(ctx context.Context, entity *Entity) error {
	rows, err := s.db.QueryContext(ctx, `SELECT display_alias FROM trend_entity_aliases WHERE entity_id = ? ORDER BY display_alias`, entity.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			rows.Close()
			return err
		}
		entity.Aliases = append(entity.Aliases, alias)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	termRows, err := s.db.QueryContext(ctx, `SELECT term FROM trend_entity_terms WHERE entity_id = ? ORDER BY term`, entity.ID)
	if err != nil {
		return err
	}
	defer termRows.Close()
	for termRows.Next() {
		var term string
		if err := termRows.Scan(&term); err != nil {
			return err
		}
		entity.Terms = append(entity.Terms, term)
	}
	return termRows.Err()
}

func hydrateEntityTimes(entity *Entity, firstSeen, lastSeen string) error {
	var err error
	entity.FirstSeen, err = time.Parse(time.RFC3339Nano, firstSeen)
	if err != nil {
		return err
	}
	entity.LastSeen, err = time.Parse(time.RFC3339Nano, lastSeen)
	return err
}

func normalizedTerms(terms []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}
	sort.Strings(out)
	return out
}

func normalizedAliases(values ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		alias := normalizedAlias(value)
		if alias == "" {
			continue
		}
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		out = append(out, alias)
	}
	return out
}

func normalizedAlias(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func newEntityID() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "tr_" + hex.EncodeToString(bytes[:]), nil
}
