package store

import (
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestReconcilePartition_SweepsDeletedKeepsSeenAndOtherPartitions(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// Project A: m1 (seen), m2 (deleted-in-remote, synced => has typed row),
	// m3 (legacy ghost: generic only, no typed row). Project B: m9 (untouched).
	// projects_id required for typed-table insert; project used for partition scoping.
	mustUpsert(t, s, "modules", `{"id":"m1","project":"A","projects_id":"A"}`)
	mustUpsert(t, s, "modules", `{"id":"m2","project":"A","projects_id":"A"}`)
	// m3 ghost: generic row only, body carries project, no typed projection.
	mustGenericOnly(t, s, "modules", "m3", `{"id":"m3","project":"A"}`)
	mustUpsert(t, s, "modules", `{"id":"m9","project":"B","projects_id":"B"}`)
	// Junction rows for m2 (must be cascaded) and m1 (must survive).
	mustExec(t, s, `CREATE TABLE IF NOT EXISTS module_issues (module_id TEXT, issue_id TEXT, project_id TEXT)`)
	mustExec(t, s, `INSERT INTO module_issues VALUES ('m2','i1','A'),('m1','i2','A')`)

	deleted, err := s.ReconcilePartition(
		"modules", "$.project", "A",
		[]string{"m1"}, // only m1 still exists remotely
		"modules",
		[]CascadeJunction{{Table: "module_issues", FKColumn: "module_id"}},
	)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if deleted != 2 { // m2 and m3
		t.Fatalf("deleted = %d, want 2 (m2 typed + m3 ghost)", deleted)
	}
	assertCount(t, s, `SELECT COUNT(*) FROM resources WHERE resource_type='modules'`, 2)   // m1, m9
	assertCount(t, s, `SELECT COUNT(*) FROM "modules"`, 2)                                   // m1, m9 typed
	assertCount(t, s, `SELECT COUNT(*) FROM resources WHERE id='m9'`, 1)                     // partition B intact
	assertCount(t, s, `SELECT COUNT(*) FROM module_issues WHERE module_id='m2'`, 0)          // cascade
	assertCount(t, s, `SELECT COUNT(*) FROM module_issues WHERE module_id='m1'`, 1)          // survives
	assertCount(t, s, `SELECT COUNT(*) FROM resources_fts WHERE rowid=?`, 0, ftsRowID("modules", "m2"))
}

func TestReconcilePartition_SkipsMalformedJSONRow(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// keep (seen) + stale (project A, unseen => victim) + a junk row whose data
	// is NOT JSON (a cached HTML/SPA error page — the prod failure class). Without
	// the json_valid CASE guard on the victim SELECT, json_extract over the junk
	// row aborts the WHOLE scan with "malformed JSON" and nothing is swept; with
	// it, the unparseable row is never a victim and the stale row still deletes.
	mustUpsert(t, s, "modules", `{"id":"m1","project":"A","projects_id":"A"}`)
	mustUpsert(t, s, "modules", `{"id":"m2","project":"A","projects_id":"A"}`)
	mustGenericOnly(t, s, "modules", "mjunk", `<!DOCTYPE html><html><body>error</body></html>`)

	deleted, err := s.ReconcilePartition(
		"modules", "$.project", "A",
		[]string{"m1"}, // only m1 still exists remotely
		"modules", nil,
	)
	if err != nil {
		t.Fatalf("reconcile (json_valid guard must swallow the junk row): %v", err)
	}
	if deleted != 1 { // m2 only; m1 seen, mjunk unparseable => never a victim
		t.Fatalf("deleted = %d, want 1 (m2 only)", deleted)
	}
	assertCount(t, s, `SELECT COUNT(*) FROM resources WHERE id='mjunk'`, 1) // junk untouched
	assertCount(t, s, `SELECT COUNT(*) FROM resources WHERE id='m1'`, 1)    // seen survives
}

func TestReconcilePartition_EmptyScopeErrors(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "data.db"))
	defer s.Close()
	if _, err := s.ReconcilePartition("modules", "$.project", "", nil, "modules", nil); err == nil {
		t.Fatal("want error on empty scope, got nil")
	}
}

func TestCascadeJunctionRegistry(t *testing.T) {
	RegisterCascadeJunction("modules", CascadeJunction{Table: "module_issues", FKColumn: "module_id"})
	got := CascadeJunctionsFor("modules")
	if len(got) != 1 || got[0].Table != "module_issues" || got[0].FKColumn != "module_id" {
		t.Fatalf("registry = %+v, want one module_issues/module_id", got)
	}
}

// mustUpsert calls s.UpsertBatch with a single JSON item. Fatals on error.
func mustUpsert(t *testing.T, s *Store, resourceType, body string) {
	t.Helper()
	_, _, err := s.UpsertBatch(resourceType, []json.RawMessage{json.RawMessage(body)})
	if err != nil {
		t.Fatalf("mustUpsert(%s): %v", resourceType, err)
	}
}

// mustGenericOnly writes a generic resources row (+ FTS) WITHOUT the typed
// table projection, simulating a legacy ghost row that has no typed row.
func mustGenericOnly(t *testing.T, s *Store, resourceType, id, body string) {
	t.Helper()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("mustGenericOnly begin: %v", err)
	}
	defer tx.Rollback()
	if err := s.upsertGenericResourceTx(tx, resourceType, id, json.RawMessage(body)); err != nil {
		t.Fatalf("mustGenericOnly upsert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("mustGenericOnly commit: %v", err)
	}
}

// mustExec runs a raw SQL statement against the store's DB. Fatals on error.
func mustExec(t *testing.T, s *Store, query string, args ...any) {
	t.Helper()
	if _, err := s.db.Exec(query, args...); err != nil {
		t.Fatalf("mustExec(%q): %v", query, err)
	}
}

// assertCount runs a COUNT query and fatals if the result doesn't match want.
func assertCount(t *testing.T, s *Store, query string, want int, args ...any) {
	t.Helper()
	var got int
	if err := s.db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatalf("assertCount(%q): %v", query, err)
	}
	if got != want {
		t.Fatalf("assertCount(%q) = %d, want %d", query, got, want)
	}
}
