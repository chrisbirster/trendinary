package history

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var externallyManagedDBs sync.Map // map[*sql.DB]struct{}

// applicationSchemaTables is the runtime/reset ownership boundary for the
// versioned migration schema. Keep this list in lockstep with migrations;
// schema_contract_test.go enforces the contract.
var applicationSchemaTables = []string{
	"signals",
	"trend_snapshots",
	"trend_chart_entries",
	"stream_cursors",
	"trend_entities",
	"trend_entity_terms",
	"trend_entity_aliases",
	"trend_signal_memberships",
	"trend_source_observations",
	"source_api_daily_quota",
	"editorial_sources",
	"content_items",
	"content_discoveries",
	"content_notes",
	"ingestion_runs",
	"newsletter_issues",
	"newsletter_issue_items",
	"quality_feedback",
	"following_radars",
	"following_follows",
	"following_baselines",
	"following_alerts",
	"following_push_subscriptions",
	"following_kv",
	"following_alert_context",
	"following_alert_feedback",
	"following_alert_delivery",
	"following_briefing_checkpoint",
}

// Explicit reset also clears legacy application-owned objects and the Goose
// migration ledger so a reset database cannot be mistaken for an up-to-date
// database on the next migration run.
var legacyApplicationSchemaTables = []string{
	"trendinary_database_resets",
	"goose_db_version",
}

func markExternallyManaged(store *Store) {
	if store == nil || store.db == nil {
		return
	}
	externallyManagedDBs.Store(store.db, struct{}{})
	// These lazy schema guards historically emitted CREATE TABLE/INDEX statements
	// on first use. Versioned migrations own schema changes now, so mark all
	// current history schema families ready when the database was opened in
	// external mode.
	cursorSchemaReady.Store(store, struct{}{})
	entitySchemaReady.Store(store, struct{}{})
	membershipSchemaReady.Store(store, struct{}{})
	propagationSchemaReady.Store(store, struct{}{})
	quotaSchemaReady.Store(store, struct{}{})
}

func IsExternallyManagedDB(db *sql.DB) bool {
	if db == nil {
		return false
	}
	_, ok := externallyManagedDBs.Load(db)
	return ok
}

func (s *Store) ExternallyManagedSchema() bool {
	return s != nil && IsExternallyManagedDB(s.db)
}

// VerifySchema is intentionally read-only. Production startup calls this after
// migrations have run and fails fast if the migration job did not leave a
// usable schema. The application process never creates, alters, or drops schema
// here.
func (s *Store) VerifySchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is required")
	}

	rows, err := s.db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return fmt.Errorf("inspect database schema: %w", err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("inspect database schema: %w", err)
		}
		found[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect database schema: %w", err)
	}

	missing := make([]string, 0)
	for _, name := range applicationSchemaTables {
		if !found[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("database schema is not migrated; missing tables: %s; run Goose migrations before starting Trendinary", strings.Join(missing, ", "))
	}

	// The membership timestamp was the last additive migration before external
	// migration ownership. Checking it protects against pointing a new binary at
	// an older schema that happens to contain all expected table names.
	columns, err := s.db.QueryContext(ctx, `PRAGMA table_info(trend_signal_memberships)`)
	if err != nil {
		return fmt.Errorf("inspect trend_signal_memberships: %w", err)
	}
	defer columns.Close()
	hasFirstObserved := false
	for columns.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := columns.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("inspect trend_signal_memberships: %w", err)
		}
		if name == "first_observed_at" {
			hasFirstObserved = true
		}
	}
	if err := columns.Err(); err != nil {
		return fmt.Errorf("inspect trend_signal_memberships: %w", err)
	}
	if !hasFirstObserved {
		return fmt.Errorf("database schema is not migrated; trend_signal_memberships.first_observed_at is missing; run Goose migrations before starting Trendinary")
	}
	return nil
}
