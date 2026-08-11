package store

import "fmt"

// Reconciliation of the local store against the live workspace.
//
// sync is otherwise append-only: an issue deleted or archived in Linear keeps
// its local row forever, and every --data-source local read then reports work
// that does not exist. This is the Go port of ~/.local/bin/linear-store-prune,
// with its two load-bearing guardrails carried over verbatim:
//
//   1. never prune against an empty live set (an auth failure, a throttled
//      response or a bad filter must not wipe the store), and
//   2. after pruning issues, issues and issues_fts must still agree.
//
// The caller owns the third guardrail: it may only pass a live set that came
// from a crawl which ran to the last page.

// prunableTables is the allowlist of typed tables a reconcile pass may delete
// from. Table names reach SQL through string concatenation, so anything absent
// from this map never gets near a statement. Two deliberate absences: the
// pp_created ledger, which records issues this CLI created and is never pruned,
// and the generic resources table, whose rows come from write-through caching
// rather than from a complete enumeration.
var prunableTables = map[string]bool{
	"teams":           true,
	"users":           true,
	"workflow_states": true,
	"issue_labels":    true,
	"projects":        true,
	"cycles":          true,
	"issues":          true,
	// The shell tables sync fills on a full crawl (GAP-038). Each one is
	// enumerated from a workspace-wide root connection that pages to
	// exhaustion, so an id absent from the live set really is gone. favorites
	// is scoped by the API to the authenticated user, which is exactly the
	// population the local table holds, so it reconciles the same way.
	"documents":          true,
	"templates":          true,
	"custom_views":       true,
	"favorites":          true,
	"project_milestones": true,
	"project_statuses":   true,
	"initiatives":        true,
	"issue_relations":    true,
}

// PrunableTable reports whether table may be reconciled by PruneMissing.
func PrunableTable(table string) bool { return prunableTables[table] }

// PruneMissing deletes every row of table whose id is absent from liveIDs and
// returns how many rows went. liveIDs MUST be the complete set of live upstream
// ids for that resource: a partial set deletes live data.
func (s *Store) PruneMissing(table string, liveIDs []string) (int, error) {
	return s.reconcileTable(table, liveIDs, true)
}

// CountMissing reports how many rows PruneMissing would delete, deleting
// nothing. Used by sync --dry-run.
func (s *Store) CountMissing(table string, liveIDs []string) (int, error) {
	return s.reconcileTable(table, liveIDs, false)
}

func (s *Store) reconcileTable(table string, liveIDs []string, del bool) (int, error) {
	if !prunableTables[table] {
		return 0, fmt.Errorf("table %q is not prunable", table)
	}
	if len(liveIDs) == 0 {
		return 0, fmt.Errorf("refusing to prune %s: the live set is empty", table)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("beginning prune transaction: %w", err)
	}
	defer tx.Rollback()

	// A temp table beats a giant IN (?, ?, ...) list: no bound-variable ceiling
	// and the primary key gives the anti-join an index. Temp tables are
	// per-connection and the whole pass runs inside one transaction, so the
	// scratch table cannot leak across pooled connections once dropped.
	if _, err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS prune_live_ids (id TEXT PRIMARY KEY)`); err != nil {
		return 0, fmt.Errorf("creating live id scratch table: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM temp.prune_live_ids`); err != nil {
		return 0, fmt.Errorf("clearing live id scratch table: %w", err)
	}
	insert, err := tx.Prepare(`INSERT OR IGNORE INTO temp.prune_live_ids(id) VALUES (?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing live id insert: %w", err)
	}
	defer insert.Close()
	loaded := 0
	for _, id := range liveIDs {
		if id == "" {
			continue
		}
		if _, err := insert.Exec(id); err != nil {
			return 0, fmt.Errorf("loading live id: %w", err)
		}
		loaded++
	}
	if loaded == 0 {
		// Same guardrail as above, reached when every supplied id was blank.
		return 0, fmt.Errorf("refusing to prune %s: the live set is empty", table)
	}

	if !del {
		var stale int
		row := tx.QueryRow(`SELECT count(*) FROM ` + table + ` WHERE id NOT IN (SELECT id FROM temp.prune_live_ids)`)
		if err := row.Scan(&stale); err != nil {
			return 0, fmt.Errorf("counting stale %s rows: %w", table, err)
		}
		if _, err := tx.Exec(`DROP TABLE IF EXISTS temp.prune_live_ids`); err != nil {
			return 0, fmt.Errorf("dropping live id scratch table: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("committing prune count: %w", err)
		}
		return stale, nil
	}

	// The issues_ad AFTER DELETE trigger keeps issues_fts consistent, so no
	// reindex is needed here. VerifyIssuesFTS checks that it actually did.
	res, err := tx.Exec(`DELETE FROM ` + table + ` WHERE id NOT IN (SELECT id FROM temp.prune_live_ids)`)
	if err != nil {
		return 0, fmt.Errorf("pruning %s: %w", table, err)
	}
	removed, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("counting pruned %s rows: %w", table, err)
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS temp.prune_live_ids`); err != nil {
		return 0, fmt.Errorf("dropping live id scratch table: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing prune of %s: %w", table, err)
	}
	return int(removed), nil
}

// VerifyIssuesFTS returns the issues row count and the issues_fts row count.
// They must match after a prune. A mismatch means the FTS triggers did not fire
// and local search is reporting rows that no longer exist.
func (s *Store) VerifyIssuesFTS() (int, int, error) {
	var issues, fts int
	if err := s.db.QueryRow(`SELECT count(*) FROM issues`).Scan(&issues); err != nil {
		return 0, 0, fmt.Errorf("counting issues: %w", err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM issues_fts`).Scan(&fts); err != nil {
		return issues, 0, fmt.Errorf("counting issues_fts: %w", err)
	}
	return issues, fts, nil
}
