package store

import (
	"encoding/json"
	"fmt"
)

// Persistence for the shell tables sync fills on a full crawl (GAP-038).
//
// migrate() has always created documents, templates, custom_views, favorites,
// project_milestones, project_statuses and initiatives as bare
// id/data/synced_at tables, and nothing ever wrote a row to any of them. Every
// --data-source local read of those resources therefore answered empty while
// the same read went live, which is the worst of both worlds: the store looks
// synced and is not.
//
// These tables carry no typed columns, so one generic writer serves all of
// them. The table name reaches SQL through string concatenation inside
// upsertEntity, so it must come from the allowlist below and never from a
// caller-supplied string. issue_relations is deliberately absent: it has typed
// columns and its own writer in relations.go.

// syncedShellTables is the allowlist of bare id/data/synced_at tables the sync
// crawl may write to. Membership here is what lets a table name be
// concatenated into a statement.
var syncedShellTables = map[string]bool{
	"documents":          true,
	"templates":          true,
	"custom_views":       true,
	"favorites":          true,
	"project_milestones": true,
	"project_statuses":   true,
	"initiatives":        true,
}

// UpsertShellRow writes one API node into the named shell table, replacing
// any row with the same id. data is the full API node, so a local read
// returns the shape a live read returns.
func (s *Store) UpsertShellRow(table, id string, data json.RawMessage) error {
	if !syncedShellTables[table] {
		return fmt.Errorf("table %q is not a synced shell table", table)
	}
	if id == "" {
		return fmt.Errorf("upsert %s: empty id", table)
	}
	return s.upsertEntity(table, []string{"id", "data"}, id, string(data))
}
