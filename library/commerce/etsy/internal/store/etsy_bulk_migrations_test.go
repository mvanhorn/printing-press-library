package store

import (
	"path/filepath"
	"testing"
)

func openBulkTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBulkTablesAreLazyAndJournalResumes(t *testing.T) {
	s := openBulkTestStore(t)
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('bulk_edit_jobs','bulk_edit_items','listing_snapshots')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("tables exist before first use: %d %v", n, err)
	}
	if err := s.BulkStartJob("job-1", "plan.json", 2, false); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('bulk_edit_jobs','bulk_edit_items','listing_snapshots')`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("tables after init: %d %v", n, err)
	}
	for _, k := range []string{"1|patch", "2|patch"} {
		if err := s.BulkRegisterOp(BulkOp{JobID: "job-1", OpKey: k, ListingID: 1, Kind: "patch"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.BulkMarkOp("job-1", "1|patch", BulkStatusDone, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.BulkMarkOp("job-1", "2|patch", BulkStatusFailed, "HTTP 400", 0); err != nil {
		t.Fatal(err)
	}
	// resuming: re-start + re-register must keep prior statuses
	if err := s.BulkStartJob("job-1", "plan.json", 2, false); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"1|patch", "2|patch"} {
		if err := s.BulkRegisterOp(BulkOp{JobID: "job-1", OpKey: k, ListingID: 1, Kind: "patch"}); err != nil {
			t.Fatal(err)
		}
	}
	if st, _ := s.BulkOpStatus("job-1", "1|patch"); st != BulkStatusDone {
		t.Fatalf("done op reset to %q", st)
	}
	ops, err := s.BulkJobOps("job-1")
	if err != nil || len(ops) != 2 || ops[1].Status != BulkStatusFailed || ops[1].Error != "HTTP 400" || ops[1].Attempts != 1 {
		t.Fatalf("ops = %+v %v", ops, err)
	}
	if err := s.BulkMarkOp("job-1", "nope", BulkStatusDone, "", 0); err == nil {
		t.Fatal("marking an unregistered op succeeded")
	}
	// reset discards the journal
	if err := s.BulkStartJob("job-1", "plan.json", 2, true); err != nil {
		t.Fatal(err)
	}
	if ops, _ := s.BulkJobOps("job-1"); len(ops) != 0 {
		t.Fatalf("reset kept %d ops", len(ops))
	}
	if st, _ := s.BulkOpStatus("job-1", "1|patch"); st != "" {
		t.Fatalf("status after reset = %q", st)
	}
}

func TestListingSnapshotsHistoryOrderAndBodies(t *testing.T) {
	s := openBulkTestStore(t)
	a, err := s.SaveListingSnapshot(10, "live", `{"v":1}`, `{"inv":1}`, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.SaveListingSnapshot(10, "live", `{"v":2}`, "", "")
	if _, err := s.SaveListingSnapshot(11, "store", `{"x":1}`, "", ""); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListListingSnapshots(10, 10, true)
	if err != nil || len(rows) != 2 || rows[0].ID != b || rows[1].ID != a || rows[1].InventoryJSON != `{"inv":1}` {
		t.Fatalf("rows = %+v %v", rows, err)
	}
	if rows, _ = s.ListListingSnapshots(10, 10, false); rows[0].ListingJSON != "" {
		t.Fatalf("bodies leaked: %+v", rows[0])
	}
	if rows, _ = s.ListListingSnapshots(0, 10, false); len(rows) != 3 {
		t.Fatalf("all = %d", len(rows))
	}
	if rows, _ = s.ListListingSnapshots(99, 10, false); len(rows) != 0 {
		t.Fatalf("unknown listing = %d", len(rows))
	}
}
