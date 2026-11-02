package govcache

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
	_ "modernc.org/sqlite"
	"path/filepath"
	"strings"
	"testing"
)

func fixture() govdata.Snapshot {
	return govdata.Snapshot{ID: "water-week1", Query: "water", Source: "both", CapturedAt: "2026-11-01T12:00:00Z", Complete: false, Datasets: []govdata.Dataset{{Source: "california", ID: "ca-1", Title: "Water", Publisher: "DWR", License: "Public Domain", Modified: "2026-10-01", Raw: json.RawMessage(`{"title":"Water","id":"ca-1"}`), Resources: []govdata.Resource{{ID: "r1", URL: "https://agency.gov/water.csv", Format: "CSV"}, {ID: "r2", URL: "https://agency.gov/water.csv?variant=2", Format: "CSV"}}}, {Source: "federal", ID: "us-1", Title: "Water national catalog", Resources: []govdata.Resource{{ID: "0", URL: "https://agency.gov/water.csv", Format: "csv"}}}}}
}
func TestImmutableScopedSnapshots(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "evidence.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	s := fixture()
	s.Scope = map[string]any{"limit": 2, "format": "CSV"}
	if err = Save(ctx, db, s); err != nil {
		t.Fatal(err)
	}
	if err = Save(ctx, db, s); err == nil {
		t.Fatal("duplicate snapshot ID overwrote immutable evidence")
	}
	s.ID = "week2"
	s.Datasets = s.Datasets[:1]
	s.Datasets[0].Title = "Changed title"
	if err = Save(ctx, db, s); err != nil {
		t.Fatal(err)
	}
	a, err := Load(ctx, db, "water-week1")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Datasets) != 2 || a.Datasets[0].Title != "Water" || a.Scope["format"] != "CSV" {
		t.Fatalf("original evidence changed: %+v", a)
	}
	b, err := Load(ctx, db, "week2")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Datasets) != 1 {
		t.Fatal("snapshot membership leaked")
	}
	ls, err := List(ctx, db)
	if err != nil || len(ls) != 2 {
		t.Fatalf("list: %v %v", ls, err)
	}
	if _, err = Load(ctx, db, "absent"); err == nil || !strings.Contains(err.Error(), "--save") {
		t.Fatalf("missing evidence error not actionable: %v", err)
	}
	bad := fixture()
	bad.ID = "sec"
	bad.Datasets[0].Source = "sec"
	if err = Save(ctx, db, bad); err == nil {
		t.Fatal("SEC mixed into catalog snapshot")
	}
}
func TestExactCrossSourceOverlap(t *testing.T) {
	s := fixture()
	o := Overlap(s)
	if len(o) != 1 || o[0].URL != "https://agency.gov/water.csv" || len(o[0].References) != 2 {
		t.Fatalf("incorrect exact overlap: %+v", o)
	}
	s.Datasets = s.Datasets[:1]
	s.Datasets[0].Resources = append(s.Datasets[0].Resources, s.Datasets[0].Resources[0])
	if len(Overlap(s)) != 0 {
		t.Fatal("same-source duplicates reported as cross-source overlap")
	}
}
func TestSnapshotDiffDoesNotClaimDeletion(t *testing.T) {
	a := fixture()
	b := fixture()
	b.ID = "week2"
	b.Datasets = append([]govdata.Dataset{}, b.Datasets[:1]...)
	b.Datasets[0].Title = "Updated"
	b.Datasets[0].Resources = []govdata.Resource{{ID: "r1", URL: "https://agency.gov/new.csv", Format: "CSV"}}
	changes := Diff(a, b)
	if len(changes) != 2 {
		t.Fatalf("changes %+v", changes)
	}
	if changes[1].Kind != "absent_from_snapshot" {
		t.Fatalf("absence misrepresented: %+v", changes)
	}
	fields := map[string]bool{}
	for _, c := range changes[0].Fields {
		fields[c.Field] = true
	}
	if !fields["title"] || !fields["resources"] {
		t.Fatalf("missing field diffs %+v", changes)
	}
	if len(Diff(a, a)) != 0 {
		t.Fatal("self diff nonempty")
	}
}
func TestCanonicalFingerprintAndExplicitSelection(t *testing.T) {
	s := fixture()
	d := s.Datasets[0]
	x, err := Fingerprint(d)
	if err != nil {
		t.Fatal(err)
	}
	d.Raw = json.RawMessage(`{"id":"ca-1", "title":"Water"}`)
	y, err := Fingerprint(d)
	if err != nil || x != y {
		t.Fatalf("key order changed fingerprint: %s %s %v", x, y, err)
	}
	d.Raw = json.RawMessage(`{"title":"Changed","id":"ca-1"}`)
	z, _ := Fingerprint(d)
	if z == x {
		t.Fatal("metadata edit did not change fingerprint")
	}
	locked, err := Lock(s, []string{"federal:us-1:0", "california:ca-1:r1"})
	if err != nil || len(locked) != 2 || locked[0].Source != "california" || locked[0].MetadataSHA256 != x {
		t.Fatalf("lock %+v %v", locked, err)
	}
	if _, err = Lock(s, []string{"california:ca-1:nope"}); err == nil {
		t.Fatal("missing selection accepted")
	}
}
func TestAuditAndCoverageStayWithinScope(t *testing.T) {
	s := fixture()
	a := Audit(s)
	codes := map[string]bool{}
	for _, f := range a {
		if f.Dataset == "federal:us-1" {
			codes[f.Code] = true
		}
	}
	if !codes["missing_license"] || !codes["missing_modified"] || !codes["missing_publisher"] {
		t.Fatalf("missing audit codes: %+v", a)
	}
	c := Coverage(s)
	if len(c) != 2 || c[0].Source != "california" || c[0].Resources != 2 || c[0].Datasets != 1 || c[1].Format != "CSV" {
		t.Fatalf("incorrect scoped coverage: %+v", c)
	}
	s.Datasets = s.Datasets[:1]
	if len(Coverage(s)) != 1 {
		t.Fatal("coverage included absent source")
	}
	s.Datasets[0].Resources = nil
	c = Coverage(s)
	if len(c) != 1 || c[0].Format != "UNDECLARED" || c[0].Resources != 0 || c[0].Datasets != 1 {
		t.Fatalf("empty resource evidence lost: %+v", c)
	}
}

func TestFingerprintPreservesLargeJSONNumbers(t *testing.T) {
	d := govdata.Dataset{Raw: json.RawMessage(`{"n":9007199254740992}`)}
	a, e := Fingerprint(d)
	if e != nil {
		t.Fatal(e)
	}
	d.Raw = json.RawMessage(`{"n":9007199254740993}`)
	b, e := Fingerprint(d)
	if e != nil || a == b {
		t.Fatalf("large integer precision lost %v", e)
	}
}
