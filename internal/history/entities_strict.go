package history

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	maxStrictEntityTerms   = 24
	maxStrictEntityAliases = 24
)

// ResolveEntityStrict is the production trend-identity resolver. Unlike the
// legacy resolver, it does not let an entity grow an unbounded vocabulary over
// time. Exact canonical names are stable; wording changes require strong term
// agreement, and entities that are already suspiciously broad are quarantined
// from fuzzy matching. Successful matches replace the active terms/aliases
// rather than unioning them forever.
func (s *Store) ResolveEntityStrict(ctx context.Context, name, clusterKey string, terms []string, observedAt time.Time) (Entity, error) {
	if err := s.ensureEntitySchema(ctx); err != nil {
		return Entity{}, err
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	terms = normalizedTerms(terms)
	nameAlias := normalizedAlias(name)

	entity, found, err := s.findStrictEntityByAlias(ctx, nameAlias, terms)
	if err != nil {
		return Entity{}, err
	}
	if !found && len(terms) >= 2 {
		entity, found, err = s.findStrictEntityByTerms(ctx, terms)
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
	if err := s.touchEntityStrict(ctx, entity, name, terms, observedAt); err != nil {
		return Entity{}, err
	}
	return s.EntityByID(ctx, entity.ID)
}

func (s *Store) findStrictEntityByAlias(ctx context.Context, alias string, terms []string) (Entity, bool, error) {
	if alias == "" {
		return Entity{}, false, nil
	}
	var id string
	err := s.db.QueryRowContext(ctx, `
SELECT entity_id FROM trend_entity_aliases WHERE alias_key = ? ORDER BY entity_id LIMIT 1`, alias).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Entity{}, false, nil
	}
	if err != nil {
		return Entity{}, false, err
	}
	entity, err := s.EntityByID(ctx, id)
	if err != nil {
		return Entity{}, false, err
	}
	if normalizedAlias(entity.Name) == alias {
		return entity, true, nil
	}
	if strictEntityMatch(entity, terms) {
		return entity, true, nil
	}
	return Entity{}, false, nil
}

func (s *Store) findStrictEntityByTerms(ctx context.Context, terms []string) (Entity, bool, error) {
	if len(terms) < 2 {
		return Entity{}, false, nil
	}
	placeholders := make([]string, len(terms))
	args := make([]any, len(terms))
	for i, term := range terms {
		placeholders[i] = "?"
		args[i] = term
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT entity_id, COUNT(*) AS hits
FROM trend_entity_terms
WHERE term IN (`+strings.Join(placeholders, ",")+`)
GROUP BY entity_id
ORDER BY hits DESC, entity_id
LIMIT 8`, args...)
	if err != nil {
		return Entity{}, false, err
	}
	type candidateRef struct {
		id   string
		hits int
	}
	refs := make([]candidateRef, 0, 8)
	for rows.Next() {
		var ref candidateRef
		if err := rows.Scan(&ref.id, &ref.hits); err != nil {
			rows.Close()
			return Entity{}, false, err
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Entity{}, false, err
	}
	if err := rows.Close(); err != nil {
		return Entity{}, false, err
	}

	type candidate struct {
		entity Entity
		hits   int
	}
	matches := make([]candidate, 0, len(refs))
	for _, ref := range refs {
		entity, err := s.EntityByID(ctx, ref.id)
		if err != nil {
			return Entity{}, false, err
		}
		if strictEntityMatch(entity, terms) {
			matches = append(matches, candidate{entity: entity, hits: ref.hits})
		}
	}
	if len(matches) == 0 {
		return Entity{}, false, nil
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].hits == matches[j].hits {
			return matches[i].entity.ID < matches[j].entity.ID
		}
		return matches[i].hits > matches[j].hits
	})
	return matches[0].entity, true, nil
}

func strictEntityMatch(entity Entity, currentTerms []string) bool {
	if len(currentTerms) < 2 {
		return false
	}
	if len(entity.Terms) > maxStrictEntityTerms || len(entity.Aliases) > maxStrictEntityAliases {
		return false
	}
	seen := make(map[string]struct{}, len(entity.Terms))
	for _, term := range entity.Terms {
		seen[strings.ToLower(strings.TrimSpace(term))] = struct{}{}
	}
	hits := 0
	for _, term := range currentTerms {
		if _, ok := seen[term]; ok {
			hits++
		}
	}
	required := (2*len(currentTerms) + 2) / 3 // ceil(2n/3)
	if required < 2 {
		required = 2
	}
	return hits >= required
}

func (s *Store) touchEntityStrict(ctx context.Context, entity Entity, currentName string, terms []string, observedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE trend_entities SET last_seen = ? WHERE id = ?`, observedAt.UTC().Format(time.RFC3339Nano), entity.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM trend_entity_terms WHERE entity_id = ?`, entity.ID); err != nil {
		return err
	}
	for _, term := range normalizedTerms(terms) {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO trend_entity_terms (entity_id, term) VALUES (?, ?)`, entity.ID, term); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM trend_entity_aliases WHERE entity_id = ?`, entity.ID); err != nil {
		return err
	}
	aliases := normalizedAliases(entity.Name, currentName)
	for _, alias := range aliases {
		display := alias
		if normalizedAlias(entity.Name) == alias && strings.TrimSpace(entity.Name) != "" {
			display = strings.TrimSpace(entity.Name)
		}
		if normalizedAlias(currentName) == alias && strings.TrimSpace(currentName) != "" {
			display = strings.TrimSpace(currentName)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO trend_entity_aliases (entity_id, alias_key, display_alias) VALUES (?, ?, ?)`, entity.ID, alias, display); err != nil {
			return err
		}
	}
	return tx.Commit()
}
